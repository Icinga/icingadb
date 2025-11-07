// #nosec G404 -- Allow math/rand for the tests
package notifications

import (
	"cmp"
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icingadb/pkg/icingadb/history"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func Test_redisStreamIdToMs(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMs  int64
		wantSeq int64
		wantErr bool
	}{
		{
			name:  "epoch",
			input: "0-0",
		},
		{
			name:   "valid",
			input:  "1761658169701-0",
			wantMs: 1761658169701,
		},
		{
			name:    "valid sequence",
			input:   "1761658169701-23",
			wantMs:  1761658169701,
			wantSeq: 23,
		},
		{
			name:    "invalid format",
			input:   "23-42-23",
			wantErr: true,
		},
		{
			name:    "missing first part",
			input:   "-23",
			wantErr: true,
		},
		{
			name:    "missing second part",
			input:   "23-",
			wantErr: true,
		},
		{
			name:    "only dash",
			input:   "-",
			wantErr: true,
		},
		{
			name:    "just invalid",
			input:   "oops",
			wantErr: true,
		},
		{
			name:    "invalid field types",
			input:   "0x23-0x42",
			wantErr: true,
		},
		{
			name:    "number too big",
			input:   "22222222222222222222222222222222222222222222222222222222222222222222222222222222222222222-0",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMs, gotSeq, err := parseRedisStreamId(tt.input)
			require.Equal(t, tt.wantErr, err != nil, "error differs %v", err)
			require.Equal(t, tt.wantMs, gotMs, "ms from Redis Stream ID differs")
			require.Equal(t, tt.wantSeq, gotSeq, "seq from Redis Stream ID differs")
		})
	}
}

func Test_streamSorterSubmissions(t *testing.T) {
	mkSubmitTime := func(offset int) time.Time {
		return time.Date(2009, 11, 10, 23, 0, 0, offset, time.UTC)
	}
	submissions := streamSorterSubmissions{
		{streamIdMs: 0, streamIdSeq: 0, submitTime: mkSubmitTime(0)},
		{streamIdMs: 1, streamIdSeq: 0, submitTime: mkSubmitTime(0)},
		{streamIdMs: 1, streamIdSeq: 1, submitTime: mkSubmitTime(0)},
		{streamIdMs: 2, streamIdSeq: 0, submitTime: mkSubmitTime(0)},
		{streamIdMs: 2, streamIdSeq: 0, submitTime: mkSubmitTime(1)},
		{streamIdMs: 3, streamIdSeq: 0, submitTime: mkSubmitTime(0)},
		{streamIdMs: 3, streamIdSeq: 1, submitTime: mkSubmitTime(0)},
		{streamIdMs: 3, streamIdSeq: 1, submitTime: mkSubmitTime(1)},
		{streamIdMs: 3, streamIdSeq: 1, submitTime: mkSubmitTime(2)},
	}

	submissionsRand := make(streamSorterSubmissions, 0, len(submissions))
	for _, i := range rand.Perm(len(submissions)) {
		submissionsRand = append(submissionsRand, submissions[i])
	}

	sort.Sort(submissionsRand)
	require.Equal(t, submissions, submissionsRand)
}

func TestStreamSorter(t *testing.T) {
	tests := []struct {
		name                   string
		messages               int
		producers              int
		producersEarlyClose    int
		callbackMaxDelayMs     int
		callbackSuccessPercent int
		expectTimeout          bool
		outMaxDelayMs          int
	}{
		{
			name:                   "baseline",
			messages:               10,
			producers:              1,
			callbackSuccessPercent: 100,
		},
		{
			name:                   "simple",
			messages:               100,
			producers:              10,
			callbackSuccessPercent: 100,
		},
		{
			name:                   "many producers",
			messages:               100,
			producers:              100,
			callbackSuccessPercent: 100,
		},
		{
			name:                   "many messages",
			messages:               1000,
			producers:              10,
			callbackSuccessPercent: 100,
		},
		{
			name:                   "callback a bit unreliable",
			messages:               50,
			producers:              10,
			callbackSuccessPercent: 70,
		},
		{
			name:                   "callback coin flip",
			messages:               50,
			producers:              10,
			callbackSuccessPercent: 50,
		},
		{
			name:                   "callback unreliable",
			messages:               25,
			producers:              5,
			callbackSuccessPercent: 30,
		},
		{
			name:                   "callback total rejection",
			messages:               10,
			producers:              1,
			callbackSuccessPercent: 0,
			expectTimeout:          true,
		},
		{
			name:                   "callback slow",
			messages:               100,
			producers:              10,
			callbackMaxDelayMs:     3000,
			callbackSuccessPercent: 100,
		},
		{
			name:                   "out slow",
			messages:               100,
			producers:              10,
			callbackSuccessPercent: 100,
			outMaxDelayMs:          1000,
		},
		{
			name:                   "producer out early close",
			messages:               100,
			producers:              10,
			producersEarlyClose:    5,
			callbackMaxDelayMs:     1000,
			callbackSuccessPercent: 100,
		},
		{
			name:                   "pure chaos",
			messages:               50,
			producers:              10,
			callbackMaxDelayMs:     3000,
			callbackSuccessPercent: 50,
			outMaxDelayMs:          1000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Callback functions after reordering
			var (
				callbackCollection      []string
				callbackCollectionMutex sync.Mutex
				callbackFn              = func(msg redis.XMessage, _ string) bool {
					if tt.callbackMaxDelayMs > 0 {
						time.Sleep(time.Duration(rand.Int63n(int64(tt.callbackMaxDelayMs))) * time.Millisecond)
					}

					if rand.Int63n(100)+1 > int64(tt.callbackSuccessPercent) {
						return false
					}

					callbackCollectionMutex.Lock()
					defer callbackCollectionMutex.Unlock()
					callbackCollection = append(callbackCollection, msg.ID)
					return true
				}
			)

			// Out channel after reordering and callback
			var (
				outCounterCh = make(chan struct{})
				outConsumer  = func(out chan redis.XMessage) {
					for {
						if tt.outMaxDelayMs > 0 {
							time.Sleep(time.Duration(rand.Int63n(int64(tt.outMaxDelayMs))) * time.Millisecond)
						}

						_, ok := <-out
						if !ok {
							return
						}
						outCounterCh <- struct{}{}
					}
				}
			)

			// Decreasing counter for messages to be sent
			var (
				inCounter      = tt.messages
				inCounterMutex sync.Mutex
			)

			sorter := NewStreamSorter(
				t.Context(),
				logging.NewLogger(zaptest.NewLogger(t).Sugar(), time.Second),
				callbackFn)
			sorter.isVerbose = true

			for i := range tt.producers {
				earlyClose := i < tt.producersEarlyClose

				in := make(chan redis.XMessage)
				out := make(chan redis.XMessage)
				go func() {
					require.NoError(t, sorter.PipelineFunc(context.Background(), history.Sync{}, "", in, out))
				}()

				if !earlyClose {
					defer close(in) // no leakage, general cleanup
				}

				go func() {
					for {
						time.Sleep(time.Duration(rand.Int63n(250)) * time.Millisecond)

						inCounterMutex.Lock()
						isFin := inCounter <= 0
						if !isFin {
							inCounter--
						}
						inCounterMutex.Unlock()

						if isFin {
							return
						}

						ms := time.Now().UnixMilli() + rand.Int63n(2_000) - 1_000
						seq := rand.Int63n(100)

						// Add 10% time travelers
						if rand.Int63n(10) == 9 {
							distanceMs := int64(1_500)
							if rand.Int63n(2) > 0 {
								// Don't go back too far. Otherwise, elements would be out of order. Three seconds max.
								ms -= distanceMs
							} else {
								ms += distanceMs
							}
						}

						msg := redis.XMessage{ID: fmt.Sprintf("%d-%d", ms, seq)}
						in <- msg

						// 25% chance of closing for early closing producers
						if earlyClose && rand.Int63n(4) == 3 {
							close(in)
							t.Log("closed producer early")
							return
						}
					}
				}()

				go outConsumer(out)
			}

			var outCounter int
		breakFor:
			for {
				select {
				case <-outCounterCh:
					outCounter++
					if outCounter == tt.messages {
						break breakFor
					}

				case <-time.After(2 * time.Minute):
					if tt.expectTimeout {
						return
					}
					t.Fatalf("Collecting messages timed out after receiving %d out of %d messages",
						outCounter, tt.messages)
				}
			}
			if tt.expectTimeout {
				t.Fatal("Timeout was expected")
			}

			callbackCollectionMutex.Lock()
			for i := 0; i < len(callbackCollection)-1; i++ {
				parse := func(id string) (int64, int64) {
					parts := strings.Split(id, "-")
					ms, err1 := strconv.ParseInt(parts[0], 10, 64)
					seq, err2 := strconv.ParseInt(parts[1], 10, 64)
					require.NoError(t, cmp.Or(err1, err2))
					return ms, seq
				}

				a, b := callbackCollection[i], callbackCollection[i+1]
				aMs, aSeq := parse(a)
				bMs, bSeq := parse(b)

				switch {
				case aMs < bMs:
				case aMs == bMs:
					if aSeq > bSeq {
						t.Errorf("collection in wrong order: %q before %q", a, b)
					}
				case aMs > bMs:
					t.Errorf("collection in wrong order: %q before %q", a, b)
				}
			}
			callbackCollectionMutex.Unlock()
		})
	}
}
