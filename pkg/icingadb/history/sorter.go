package history

import (
	"context"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"math"
	"regexp"
	"sort"
	"strconv"
	"time"
)

// parseRedisStreamId parses a Redis Stream ID and returns the timestamp in ms and the sequence number, or an error.
func parseRedisStreamId(redisStreamId string) (int64, int64, error) {
	re := regexp.MustCompile(`^(\d+)-(\d+)$`)
	matches := re.FindStringSubmatch(redisStreamId)
	if len(matches) != 3 {
		return 0, 0, errors.Errorf("value %q does not satisfy Redis Stream ID regex", redisStreamId)
	}

	ms, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, 0, errors.Wrapf(
			err,
			"timestamp part of the Redis Stream ID %q cannot be parsed to int", redisStreamId)
	}

	seq, err := strconv.ParseInt(matches[2], 10, 64)
	if err != nil {
		return 0, 0, errors.Wrapf(
			err,
			"sequence number of the Redis Stream ID %q cannot be parsed to int", redisStreamId)
	}

	return ms, seq, nil
}

// streamSorterSubmission is one submission to a StreamSorter, allowing to be sorted by the Redis Stream ID - both via
// timestamp and the sequence number as a fallback - as well as the submission timestamp for duplicates if milliseconds
// are not precise enough.
type streamSorterSubmission struct {
	// msg is the Redis message to be forwarded to out after this submission was sorted.
	msg  redis.XMessage
	args any
	out  chan<- redis.XMessage

	// Required for sorting.
	streamIdMs   int64 // streamIdMs is the Redis Stream ID timestamp part (milliseconds)
	streamIdSeq  int64 // streamIdSeq is the Redis Stream ID sequence number
	submitTimeNs int64 // submitTimeNs is the timestamp when the element was submitted (in nanoseconds)
}

// streamSorterSubmissions implements sort.Interface for []streamSorterSubmission.
type streamSorterSubmissions []streamSorterSubmission

func (subs streamSorterSubmissions) Len() int { return len(subs) }

func (subs streamSorterSubmissions) Swap(i, j int) { subs[i], subs[j] = subs[j], subs[i] }

func (subs streamSorterSubmissions) Less(i, j int) bool {
	a, b := subs[i], subs[j]
	if a.streamIdMs != b.streamIdMs {
		return a.streamIdMs < b.streamIdMs
	}
	if a.streamIdSeq != b.streamIdSeq {
		return a.streamIdSeq < b.streamIdSeq
	}
	return a.submitTimeNs < b.submitTimeNs
}

// StreamSorter accepts multiple [redis.XMessage] via Submit and ejects them in an ordered fashion.
//
// Internally, two goroutines are used. One collects the submissions and puts them into buckets based on the second
// of the Redis Stream ID. After three seconds, each bucket is being sorted and ejected to the other goroutine. There,
// each element is passed to the callback function in order. Only if the callback function has succeeded, it is removed
// from the top of the queue.
//
// Thus, if a message is received delayed for more than three seconds, it will be relayed out of order while an error is
// being logged. The StreamSorter is only able to ensure order to a certain degree of chaos.
//
// The callback function receives the [redis.XMessage] together with generic args passed in Submit for additional
// context. If the callback function returns true, the element will be removed from the queue. Otherwise, the element
// will be kept at top of the queue and retried next time.
type StreamSorter struct {
	logger       *logging.Logger
	callbackFn   func(redis.XMessage, any) bool
	submissionCh chan streamSorterSubmission

	// verbose implies a verbose debug logging. Don't think one want to have this outside the tests.
	verbose bool
}

// NewStreamSorter creates a StreamSorter honoring the given context and returning elements to the callback function.
func NewStreamSorter(
	ctx context.Context,
	logger *logging.Logger,
	callbackFn func(msg redis.XMessage, args any) bool,
) *StreamSorter {
	sorter := &StreamSorter{
		logger:       logger,
		callbackFn:   callbackFn,
		submissionCh: make(chan streamSorterSubmission),
	}

	_ = context.AfterFunc(ctx, func() { close(sorter.submissionCh) })

	ch := make(chan []streamSorterSubmission)
	go sorter.submissionWorker(ctx, ch)
	go sorter.queueWorker(ctx, ch)

	return sorter
}

// submissionWorker listens ton submissionCh populated by Submit, fills buckets and ejects them into out, linked to
// the queueWorker goroutine for further processing.
func (sorter *StreamSorter) submissionWorker(ctx context.Context, out chan<- []streamSorterSubmission) {
	// slots defines how many second slots should be kept for sorting
	const slots = 3
	// buckets maps timestamp in seconds to streamSorterSubmissions made within this second
	buckets := make(map[int64][]streamSorterSubmission)

	defer close(out)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case submission := <-sorter.submissionCh:
			curBucketId := time.Now().Unix()
			bucketId := submission.streamIdMs / 1_000
			if minBucketId := curBucketId - slots; bucketId < minBucketId {
				sorter.logger.Errorw("Received message with Stream ID from the far past, put in last bucket",
					zap.String("id", submission.msg.ID),
					zap.Int64("buckets-behind", minBucketId-bucketId))
				bucketId = minBucketId
			} else if bucketId > curBucketId {
				sorter.logger.Warnw("Received message with Stream ID from the future",
					zap.String("id", submission.msg.ID),
					zap.Int64("buckets-ahead", bucketId-curBucketId))
			}

			if sorter.verbose {
				sorter.logger.Debugw("Insert submission into bucket",
					zap.String("id", submission.msg.ID),
					zap.Int64("bucket-id", bucketId))
			}

			bucket, ok := buckets[bucketId]
			if !ok {
				bucket = make([]streamSorterSubmission, 0, 1)
			}
			buckets[bucketId] = append(bucket, submission)

		case <-ticker.C:
			// Search the smallest bucket ID older than slots+1 seconds by iterating over the keys. This is fast due to
			// slots being 3 and the submission code eliminates inserts from the far past. Usually, the latest bucket ID
			// should be "time.Now().Unix() - slots - 1", but I raced this with a very busy submission channel.
			bucketId := int64(math.MaxInt64)
			bucketSup := time.Now().Unix() - slots - 1
			for k := range buckets {
				if k <= bucketSup {
					bucketId = min(bucketId, k)
				}
			}

			bucket, ok := buckets[bucketId]
			if !ok {
				continue
			}
			delete(buckets, bucketId)

			sort.Sort(streamSorterSubmissions(bucket))
			out <- bucket

			if sorter.verbose {
				sorter.logger.Debugw("Ejected submission bucket to callback worker",
					zap.Int64("bucket-id", bucketId),
					zap.Int("bucket-size", len(bucket)))
			}
		}
	}
}

// queueWorker receives sorted streamSorterSubmissions from submissionWorker and forwards them to the callback.
func (sorter *StreamSorter) queueWorker(ctx context.Context, in <-chan []streamSorterSubmission) {
	// Each streamSorterSubmission received bucket-wise from in is stored in the queue slice. From there on, the slice
	// head is passed to the callback function. The queueEventCh has a buffer capacity of 1 to allow both filling and
	// consuming in the same goroutine.
	queue := make([]streamSorterSubmission, 0, 1024)
	queueEventCh := make(chan struct{}, 1)
	defer close(queueEventCh)

	// queueEvent places something in queueEventCh w/o deadlocking
	queueEvent := func() {
		// Always drain channel first. Consider positive <-queueEventCh case followed by <-in. Within <-in, a second
		// struct{}{} would be sent, effectively deadlocking.
		for len(queueEventCh) > 0 {
			<-queueEventCh
		}
		queueEventCh <- struct{}{}
	}

	// The actual callback function is executed concurrently as it might block longer than expected. A blocking select
	// would result in the queue not being populated, effectively blocking the submissionWorker. Thus, the callbackFn is
	// started in a goroutine, signaling back its success status via callbackCh. If no callback is active, the channel
	// is nil. Furthermore, an exponential backoff for sequentially failing callbacks is in place.
	const callbackMaxDelay = 10 * time.Second
	var callbackDelay time.Duration
	var callbackCh chan bool
	callbackFn := func(submission streamSorterSubmission) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(callbackDelay):
		}

		start := time.Now()
		success := sorter.callbackFn(submission.msg, submission.args)
		if success {
			submission.out <- submission.msg
			callbackDelay = 0
		} else {
			callbackDelay = min(2*max(time.Millisecond, callbackDelay), callbackMaxDelay)
		}

		if sorter.verbose {
			sorter.logger.Debugw("Callback finished",
				zap.String("id", submission.msg.ID),
				zap.Bool("success", success),
				zap.Duration("duration", time.Since(start)),
				zap.Duration("next-delay", callbackDelay))
		}

		callbackCh <- success
	}

	for {
		select {
		case <-ctx.Done():
			return

		case submissions, ok := <-in:
			if !ok {
				return
			}

			queue = append(queue, submissions...)
			queueEvent()

			if sorter.verbose {
				sorter.logger.Debugw("Queue worker received new submissions",
					zap.Int("queue-size", len(queue)))
			}

		case <-queueEventCh:
			if len(queue) == 0 {
				continue
			}

			if callbackCh != nil {
				continue
			}
			callbackCh = make(chan bool)
			go callbackFn(queue[0])

		case success := <-callbackCh:
			if success {
				queue = queue[1:]
			}

			close(callbackCh)
			callbackCh = nil

			if len(queue) > 0 {
				queueEvent()
			} else {
				if sorter.verbose {
					sorter.logger.Debug("Queue worker finished processing queue")
				}
			}
		}
	}
}

// Submit a [redis.XMessage] to the StreamSorter.
//
// After the message was sorted and successfully passed to the callback including the optional args, it will be
// forwarded to the out channel.
//
// This method returns an error for malformed Redis Stream IDs or if the internal submission channel blocks for over a
// second. Usually, this both should not happen.
func (sorter *StreamSorter) Submit(msg redis.XMessage, args any, out chan<- redis.XMessage) error {
	ms, seq, err := parseRedisStreamId(msg.ID)
	if err != nil {
		return errors.Wrap(err, "cannot parse Redis Stream ID")
	}

	submission := streamSorterSubmission{
		msg:          msg,
		args:         args,
		out:          out,
		streamIdMs:   ms,
		streamIdSeq:  seq,
		submitTimeNs: time.Now().UnixNano(),
	}

	select {
	case sorter.submissionCh <- submission:
		return nil

	case <-time.After(time.Second):
		return errors.New("submission timed out")
	}
}
