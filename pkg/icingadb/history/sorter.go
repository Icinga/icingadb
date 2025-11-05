package history

import (
	"container/heap"
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"strconv"
	"strings"
	"time"
)

// parseRedisStreamId parses a Redis Stream ID and returns the timestamp in ms and the sequence number, or an error.
func parseRedisStreamId(redisStreamId string) (int64, int64, error) {
	dashPos := strings.IndexRune(redisStreamId, '-')
	if dashPos <= 0 {
		return 0, 0, errors.Errorf("value %q does not satisfy Redis Stream ID pattern", redisStreamId)
	}

	ms, err := strconv.ParseInt(redisStreamId[:dashPos], 10, 64)
	if err != nil {
		return 0, 0, errors.Wrapf(
			err,
			"timestamp part of the Redis Stream ID %q cannot be parsed to int", redisStreamId)
	}

	seq, err := strconv.ParseInt(redisStreamId[dashPos+1:], 10, 64)
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
	streamIdMs  int64     // streamIdMs is the Redis Stream ID timestamp part (milliseconds)
	streamIdSeq int64     // streamIdSeq is the Redis Stream ID sequence number
	submitTime  time.Time // submitTime is the timestamp when the element was submitted
}

// MarshalLogObject implements [zapcore.ObjectMarshaler].
func (sub streamSorterSubmission) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddInt64("redis-id-ms", sub.streamIdMs)
	encoder.AddInt64("redis-id-seq", sub.streamIdSeq)
	encoder.AddTime("submit-time", sub.submitTime)

	return nil
}

// streamSorterSubmissions implements [heap.Interface] for []streamSorterSubmission.
type streamSorterSubmissions []streamSorterSubmission

// Len implements [sort.Interface] required by [heap.Interface].
func (subs streamSorterSubmissions) Len() int { return len(subs) }

// Swap implements [sort.Interface] required by [heap.Interface].
func (subs streamSorterSubmissions) Swap(i, j int) { subs[i], subs[j] = subs[j], subs[i] }

// Less implements [sort.Interface] required by [heap.Interface].
func (subs streamSorterSubmissions) Less(i, j int) bool {
	a, b := subs[i], subs[j]
	if a.streamIdMs != b.streamIdMs {
		return a.streamIdMs < b.streamIdMs
	}
	if a.streamIdSeq != b.streamIdSeq {
		return a.streamIdSeq < b.streamIdSeq
	}
	return a.submitTime.Before(b.submitTime)
}

// Push implements [heap.Interface].
func (subs *streamSorterSubmissions) Push(x any) {
	sub, ok := x.(streamSorterSubmission)
	if !ok {
		panic(fmt.Sprintf("streamSorterSubmissions.Push received x of %T", x))
	}

	*subs = append(*subs, sub)
}

// Pop implements [heap.Interface].
func (subs *streamSorterSubmissions) Pop() any {
	old := *subs
	n := len(old)
	x := old[n-1]
	*subs = old[0 : n-1]
	return x
}

// StreamSorter accepts multiple [redis.XMessage] via Submit and ejects them in an ordered fashion.
//
// Internally, two goroutines are used. The first one collects the submissions and sorts them into a heap based on the
// Redis Stream ID. After being in the heap for at least three seconds, a submission is forwarded to the other
// goroutine. There, each element is passed to the callback function in order. Only if the callback function has
// succeeded, it is removed from the top of the queue.
//
// Thus, if a message is received delayed for more than three seconds, it will be relayed out of order. The StreamSorter
// is only able to ensure order to a certain degree of chaos.
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

	ch := make(chan streamSorterSubmission)
	go sorter.submissionWorker(ctx, ch)
	go sorter.queueWorker(ctx, ch)

	return sorter
}

// submissionWorker listens ton submissionCh populated by Submit, fills the heap and ejects streamSorterSubmissions into
// out, linked to the queueWorker goroutine for further processing.
func (sorter *StreamSorter) submissionWorker(ctx context.Context, out chan<- streamSorterSubmission) {
	defer close(out)

	// When a streamSorterSubmission is created in the Submit method, the current time.Time is added to the struct.
	// Only if the submission was at least three seconds (submissionMinAge) ago, a popped submission from the heap will
	// be forwarded to the other goroutine for future processing.
	const submissionMinAge = 3 * time.Second
	submissionHeap := &streamSorterSubmissions{}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case sub := <-sorter.submissionCh:
			if sorter.verbose {
				sorter.logger.Debugw("Push submission to heap", zap.Object("submission", sub))
			}

			heap.Push(submissionHeap, sub)

		case <-ticker.C:
			start := time.Now()
			submissionCounter := 0

			for submissionHeap.Len() > 0 {
				x := heap.Pop(submissionHeap)
				sub, ok := x.(streamSorterSubmission)
				if !ok {
					panic(fmt.Sprintf("invalid type %T from submission heap", x))
				}

				if time.Since(sub.submitTime) < submissionMinAge {
					if sorter.verbose {
						sorter.logger.Debugw("Stopped popping heap as submission is not old enough",
							zap.Object("submission", sub),
							zap.Int("submissions", submissionCounter),
							zap.Duration("duration", time.Since(start)))
					}

					heap.Push(submissionHeap, sub)
					break
				}

				out <- sub
				submissionCounter++
			}

			if sorter.verbose && submissionCounter > 0 {
				sorter.logger.Debugw("Ejected submissions to callback worker",
					zap.Int("submissions", submissionCounter),
					zap.Duration("duration", time.Since(start)))
			}
		}
	}
}

// queueWorker receives sorted streamSorterSubmissions from submissionWorker and forwards them to the callback.
func (sorter *StreamSorter) queueWorker(ctx context.Context, in <-chan streamSorterSubmission) {
	// Each streamSorterSubmission received from "in" is stored in the queue slice. From there on, the slice head is
	// passed to the callback function. The queueEventCh has a buffer capacity of 1 to allow both filling and consuming
	// in the same goroutine.
	queue := make([]streamSorterSubmission, 0, 1024)
	queueEventCh := make(chan struct{}, 1)
	defer close(queueEventCh)

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
		if len(queue) > 0 && callbackCh == nil {
			callbackCh = make(chan bool)
			go callbackFn(queue[0])
		}

		select {
		case <-ctx.Done():
			return

		case sub, ok := <-in:
			if !ok {
				return
			}

			queue = append(queue, sub)

			if sorter.verbose {
				sorter.logger.Debugw("Queue worker received new submission",
					zap.Object("submission", sub),
					zap.Int("queue-size", len(queue)))
			}

		case success := <-callbackCh:
			if success {
				queue = queue[1:]
			}

			close(callbackCh)
			callbackCh = nil

			if sorter.verbose && len(queue) == 0 {
				sorter.logger.Debug("Queue worker finished processing queue")
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
		msg:         msg,
		args:        args,
		out:         out,
		streamIdMs:  ms,
		streamIdSeq: seq,
		submitTime:  time.Now(),
	}

	select {
	case sorter.submissionCh <- submission:
		return nil

	case <-time.After(time.Second):
		return errors.New("submission timed out")
	}
}
