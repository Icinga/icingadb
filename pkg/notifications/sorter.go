package notifications

import (
	"container/heap"
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icingadb/pkg/icingadb/history"
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
	msg redis.XMessage
	key string
	out chan<- redis.XMessage

	// Required for sorting.
	streamIdMs  int64     // streamIdMs is the Redis Stream ID timestamp part (milliseconds)
	streamIdSeq int64     // streamIdSeq is the Redis Stream ID sequence number
	submitTime  time.Time // submitTime is the timestamp when the element was submitted
}

// MarshalLogObject implements [zapcore.ObjectMarshaler].
func (sub *streamSorterSubmission) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddInt64("redis-id-ms", sub.streamIdMs)
	encoder.AddInt64("redis-id-seq", sub.streamIdSeq)
	encoder.AddTime("submit-time", sub.submitTime)
	encoder.AddString("out", fmt.Sprint(sub.out))

	return nil
}

// streamSorterSubmissions implements [heap.Interface] for []streamSorterSubmission.
type streamSorterSubmissions []*streamSorterSubmission

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
	sub, ok := x.(*streamSorterSubmission)
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

// Peek returns the smallest element from the heap without removing it, or nil if the heap is empty.
func (subs streamSorterSubmissions) Peek() *streamSorterSubmission {
	if len(subs) > 0 {
		return subs[0]
	} else {
		return nil
	}
}

// StreamSorter is a helper that can used to intercept messages from different history sync pipelines and passes them
// to a callback in the order given by their Redis Stream ID (sorted across all involved streams).
//
// After a message is received, it is kept in a priority queue for three seconds to wait for possible messages from
// another stream with a smaller ID. Thus, if a message is received delayed for more than three seconds, it will be
// relayed out of order. The StreamSorter is only able to ensure order to a certain degree of chaos.
//
// The callback function receives the [redis.XMessage] together with the Redis stream name (key) for additional
// context. The callback function is supposed to return true on success. Otherwise, the callback will be retried until
// it succeeds.
type StreamSorter struct {
	ctx          context.Context
	logger       *logging.Logger
	callbackFn   func(redis.XMessage, string) bool
	submissionCh chan *streamSorterSubmission

	// registerOutCh is used by PipelineFunc() to register output channels with worker()
	registerOutCh chan chan<- redis.XMessage

	// closeOutCh is used by PipelineFunc() to signal to worker() that there will be no more submissions destined for
	// that output channel and it can be closed by the worker after it processed all pending submissions for it.
	closeOutCh chan chan<- redis.XMessage

	// The following fields should only be changed for the tests.

	// callbackMaxDelay is the maximum delay for continuously failing callbacks. Defaults to 10s.
	callbackMaxDelay time.Duration
	// submissionMinAge is the minimum age for a submission before being forwarded. Defaults to 3s.
	submissionMinAge time.Duration

	// isVerbose implies a isVerbose debug logging. Don't think one want to have this outside the tests.
	isVerbose bool
}

// NewStreamSorter creates a StreamSorter honoring the given context and returning elements to the callback function.
func NewStreamSorter(
	ctx context.Context,
	logger *logging.Logger,
	callbackFn func(msg redis.XMessage, key string) bool,
) *StreamSorter {
	sorter := &StreamSorter{
		ctx:              ctx,
		logger:           logger,
		callbackFn:       callbackFn,
		submissionCh:     make(chan *streamSorterSubmission),
		registerOutCh:    make(chan chan<- redis.XMessage),
		closeOutCh:       make(chan chan<- redis.XMessage),
		callbackMaxDelay: 10 * time.Second,
		submissionMinAge: 3 * time.Second,
	}

	go sorter.worker()

	return sorter
}

// verbose produces a debug log messages if StreamSorter.isVerbose is set.
func (sorter *StreamSorter) verbose(msg string, keysAndValues ...any) {
	// When used in tests and the test context is done, using the logger results in a data race. Since there are a few
	// log messages which might occur after the test has finished, better not log at all.
	// https://github.com/uber-go/zap/issues/687#issuecomment-473382859
	if sorter.ctx.Err() != nil {
		return
	}

	if !sorter.isVerbose {
		return
	}

	sorter.logger.Debugw(msg, keysAndValues...)
}

// startCallback initiates the callback in a background goroutine and returns a channel that is closed once the callback
// has succeeded. It retries the callback with a backoff until it signal success by returning true.
func (sorter *StreamSorter) startCallback(msg redis.XMessage, key string) <-chan struct{} {
	callbackCh := make(chan struct{})

	go func() {
		defer close(callbackCh)

		callbackDelay := time.Duration(0)

		for try := 0; ; try++ {
			select {
			case <-sorter.ctx.Done():
				return
			case <-time.After(callbackDelay):
			}

			start := time.Now()
			success := sorter.callbackFn(msg, key)

			sorter.verbose("startCallback: finished executing callbackFn",
				zap.String("id", msg.ID),
				zap.Bool("success", success),
				zap.Int("try", try),
				zap.Duration("duration", time.Since(start)),
				zap.Duration("next-delay", callbackDelay))

			if success {
				return
			} else {
				callbackDelay = min(max(time.Millisecond, 2*callbackDelay), sorter.callbackMaxDelay)
			}
		}
	}()

	return callbackCh
}

// worker is the background worker, started in a goroutine from NewStreamSorter, reacts upon messages from the channels,
// and runs until the StreamSorter.ctx is done.
func (sorter *StreamSorter) worker() {
	// When a streamSorterSubmission is created in the submit method, the current time.Time is added to the struct.
	// Only if the submission was at least three seconds (submissionMinAge) ago, a popped submission from the heap will
	// be passed to startCallback in its own goroutine to execute the callback function.
	var submissionHeap streamSorterSubmissions

	// Each registered output is stored in the registeredOutputs map, mapping output channels to the following struct.
	// It counts pending submissions in the heap for each received submission from submissionCh and can be marked as
	// closed to be cleaned up after its work is done.
	type OutputState struct {
		pending int
		close   bool
	}

	registeredOutputs := make(map[chan<- redis.XMessage]*OutputState)

	// Close all registered outputs when we exit.
	defer func() {
		for out := range registeredOutputs {
			close(out)
		}
	}()

	// If a submission is currently given to the callback via startCallback, these two variables are not nil. After the
	// callback has finished, the channel will be closed.
	var runningSubmission *streamSorterSubmission
	var runningCallbackCh <-chan struct{}

	for {
		if (runningSubmission == nil) != (runningCallbackCh == nil) {
			panic(fmt.Sprintf("inconsistent state: runningSubmission=%#v and runningCallbackCh=%#v",
				runningSubmission, runningCallbackCh))
		}

		var nextSubmissionDue <-chan time.Time
		if runningCallbackCh == nil {
			if next := submissionHeap.Peek(); next != nil {
				if submissionAge := time.Since(next.submitTime); submissionAge >= sorter.submissionMinAge {
					runningCallbackCh = sorter.startCallback(next.msg, next.key)
					runningSubmission = next
					heap.Pop(&submissionHeap)
				} else {
					nextSubmissionDue = time.After(sorter.submissionMinAge - submissionAge)
				}
			}
		}

		select {
		case out := <-sorter.registerOutCh:
			sorter.verbose("worker: register output", zap.String("out", fmt.Sprint(out)))
			if _, ok := registeredOutputs[out]; ok {
				panic("attempting to register the same output channel twice")
			}
			registeredOutputs[out] = &OutputState{}
			// This function is now responsible for closing out.

		case out := <-sorter.closeOutCh:
			if state := registeredOutputs[out]; state == nil {
				panic("requested to close unknown output channel")
			} else if state.pending > 0 {
				// Still pending work, mark the output and wait for it to complete.
				state.close = true
			} else {
				// Output can be closed and unregistered immediately
				close(out)
				delete(registeredOutputs, out)
			}

		case sub := <-sorter.submissionCh:
			sorter.verbose("worker: push submission to heap", zap.Object("submission", sub))

			if state := registeredOutputs[sub.out]; state == nil {
				panic("submission for an unknown output channel")
			} else {
				state.pending++
				heap.Push(&submissionHeap, sub)
			}

		case <-nextSubmissionDue:
			// Loop start processing of the next submission.
			continue

		case <-runningCallbackCh:
			out := runningSubmission.out
			out <- runningSubmission.msg
			state := registeredOutputs[out]
			state.pending--
			if state.close && state.pending == 0 {
				close(out)
				delete(registeredOutputs, out)
			}

			runningCallbackCh = nil
			runningSubmission = nil

		case <-sorter.ctx.Done():
			return
		}
	}
}

// submit a [redis.XMessage] to the StreamSorter.
func (sorter *StreamSorter) submit(msg redis.XMessage, key string, out chan<- redis.XMessage) error {
	ms, seq, err := parseRedisStreamId(msg.ID)
	if err != nil {
		return errors.Wrap(err, "cannot parse Redis Stream ID")
	}

	submission := &streamSorterSubmission{
		msg:         msg,
		key:         key,
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

	case <-sorter.ctx.Done():
		return sorter.ctx.Err()
	}
}

// PipelineFunc implements the [history.StageFunc] type expected for a history sync pipeline stage.
//
// This method of a single StreamSorter can be inserted into multiple history sync pipelines and will forward all
// messages from in to out as expected from a pipeline stage. In between, all messages are processed by the
// StreamSorter, which correlates the messages from different pipelines and additionally passes them to a callback
// according to its specification (see the comment on the StreamSorter type).
func (sorter *StreamSorter) PipelineFunc(
	ctx context.Context,
	_ history.Sync,
	key string,
	in <-chan redis.XMessage,
	out chan<- redis.XMessage,
) error {
	// Register output channel with worker.
	select {
	case sorter.registerOutCh <- out:
		// Success, worker is now responsible for closing the channel.

	case <-ctx.Done():
		close(out)
		return ctx.Err()

	case <-sorter.ctx.Done():
		close(out)
		return sorter.ctx.Err()
	}

	// If we exit, signal to the worker that no more work for this channel will be submitted.
	defer func() {
		select {
		case sorter.closeOutCh <- out:
			// Success, worker will close the output channel eventually.

		case <-sorter.ctx.Done():
			// Worker will quit entirely, closing all output channels.
		}
	}()

	for {
		select {
		case msg, ok := <-in:
			if !ok {
				return nil
			}

			err := sorter.submit(msg, key, out)
			if err != nil {
				sorter.logger.Errorw("Failed to submit Redis stream event to stream sorter",
					zap.String("key", key),
					zap.Error(err))
			}

		case <-ctx.Done():
			return ctx.Err()

		case <-sorter.ctx.Done():
			return sorter.ctx.Err()
		}
	}
}
