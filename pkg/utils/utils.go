package utils

import (
	"context"
	"crypto/sha1"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/pkg/errors"
	"golang.org/x/exp/utf8string"
	"golang.org/x/sync/errgroup"
	"math"
	"strings"
	"time"
	"unicode"
)

// FromUnixMilli creates and returns a time.Time value
// from the given milliseconds since the Unix epoch ms.
func FromUnixMilli(ms int64) time.Time {
	sec, dec := math.Modf(float64(ms) / 1e3)

	return time.Unix(int64(sec), int64(dec*(1e9)))
}

// UnixMilli returns milliseconds since the Unix epoch of time t.
func UnixMilli(t time.Time) int64 {
	return t.UnixNano() / 1e6
}

// Name returns the declared name of type t.
// Name is used in combination with Key
// to automatically guess an entity's
// database table and Redis key.
func Name(t interface{}) string {
	s := strings.TrimLeft(fmt.Sprintf("%T", t), "*")

	return s[strings.LastIndex(s, ".")+1:]
}

// TableName returns the table of t.
func TableName(t interface{}) string {
	if tn, ok := t.(contracts.TableNamer); ok {
		return tn.TableName()
	} else {
		return Key(Name(t), '_')
	}
}

// Key returns the name with all Unicode letters mapped to lower case letters,
// with an additional separator in front of each original upper case letter.
func Key(name string, sep byte) string {
	r := []rune(name)
	b := strings.Builder{}
	b.Grow(len(r) + 2) // nominal 2 bytes of extra space for inserted delimiters

	b.WriteRune(unicode.ToLower(r[0]))
	for _, r := range r[1:] {
		if unicode.IsUpper(r) {
			b.WriteByte(sep)
		}

		b.WriteRune(unicode.ToLower(r))
	}

	return b.String()
}

// Timed calls the given callback with the time that has elapsed since the start.
//
// Timed should be installed by defer:
//
//  func TimedExample(logger *zap.SugaredLogger) {
//  	defer utils.Timed(time.Now(), func(elapsed time.Duration) {
//  		logger.Debugf("Executed job in %s", elapsed)
//  	})
//  	job()
//  }
func Timed(start time.Time, callback func(elapsed time.Duration)) {
	callback(time.Since(start))
}

// BatchSliceOfStrings groups the given keys into chunks of size count and streams them into a returned channel.
func BatchSliceOfStrings(ctx context.Context, keys []string, count int) <-chan []string {
	batches := make(chan []string)

	go func() {
		defer close(batches)

		for i := 0; i < len(keys); i += count {
			end := i + count
			if end > len(keys) {
				end = len(keys)
			}

			select {
			case batches <- keys[i:end]:
			case <-ctx.Done():
				return
			}
		}
	}()

	return batches
}

// IsContextCanceled returns whether the given error is context.Canceled.
func IsContextCanceled(err error) bool {
	return errors.Is(err, context.Canceled)
}

// Checksum returns the SHA-1 checksum of the data.
func Checksum(data interface{}) []byte {
	var chksm [sha1.Size]byte

	switch data := data.(type) {
	case string:
		chksm = sha1.Sum([]byte(data))
	case []byte:
		chksm = sha1.Sum(data)
	default:
		panic(fmt.Sprintf("Unable to create checksum for type %T", data))
	}

	return chksm[:]
}

// Fatal panics with the given error.
func Fatal(err error) {
	panic(err)
}

// IsDeadlock returns whether the given error signals serialization failure.
func IsDeadlock(err error) bool {
	var e *mysql.MySQLError
	if errors.As(err, &e) {
		switch e.Number {
		case 1205, 1213:
			return true
		}
	}

	return false
}

var ellipsis = utf8string.NewString("...")

// Ellipsize shortens s to <=limit runes and indicates shortening by "...".
func Ellipsize(s string, limit int) string {
	utf8 := utf8string.NewString(s)
	switch {
	case utf8.RuneCount() <= limit:
		return s
	case utf8.RuneCount() <= ellipsis.RuneCount():
		return ellipsis.String()
	default:
		return utf8.Slice(0, limit-ellipsis.RuneCount()) + ellipsis.String()
	}
}

// Periodic executes the specified callback after every tick.
// Stops the ticker when the passed context is cancelled or when the returned cancel function is called:
//
//  func Work(ctx context.Context, logger *zap.SugaredLogger) {
//  	var cnt com.Counter
//  	cancelPeriodic := utils.Periodic(ctx, time.Second*10, func(elapsed time.Duration) {
//  		logger.Debugf("Executed work with %d jobs in %s, cnt.Val(), elapsed)
//  	})
//  	defer cancelPeriodic()
//  	work(cnt)
//  }
func Periodic(ctx context.Context, tick time.Duration, callback func(elapsed time.Duration)) context.CancelFunc {
	ctx, cancelCtx := context.WithCancel(ctx)
	start := time.Now()
	ticker := time.NewTicker(tick)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				callback(time.Since(start))
			case <-ctx.Done():
				return
			}
		}
	}()

	return cancelCtx
}

func StablePeriodic(ctx context.Context, tick time.Duration, callback func(ctx context.Context, time time.Time) error) (context.CancelFunc, <-chan error) {
	ctx, cancelCtx := context.WithCancel(ctx)
	g, ctx := errgroup.WithContext(ctx)
	ticker := time.NewTicker(tick)

	g.Go(func() error {
		defer ticker.Stop()

		for {
			select {
			case t := <-ticker.C:
				if err := callback(ctx, t); err != nil {
					return err
				}

				ticker.Reset(tick)
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	return cancelCtx, com.WaitAsync(g)
}
