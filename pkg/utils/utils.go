package utils

import (
	"context"
	"crypto/sha1"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"golang.org/x/exp/utf8string"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Timed calls the given callback with the time that has elapsed since the start.
//
// Timed should be installed by defer:
//
//	func TimedExample(logger *zap.SugaredLogger) {
//		defer utils.Timed(time.Now(), func(elapsed time.Duration) {
//			logger.Debugf("Executed job in %s", elapsed)
//		})
//		job()
//	}
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

// IsDeadlock returns whether the given error signals serialization failure.
func IsDeadlock(err error) bool {
	var e *mysql.MySQLError
	if errors.As(err, &e) {
		switch e.Number {
		case 1205, 1213:
			return true
		default:
			return false
		}
	}

	var pe *pq.Error
	if errors.As(err, &pe) {
		switch pe.Code {
		case "40001", "40P01":
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

// AppName returns the name of the executable that started this program (process).
func AppName() string {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}

	return filepath.Base(exe)
}

// MaxInt returns the larger of the given integers.
func MaxInt(x, y int) int {
	if x > y {
		return x
	}

	return y
}

// IsUnixAddr indicates whether the given host string represents a Unix socket address.
//
// A host string that begins with a forward slash ('/') is considered Unix socket address.
func IsUnixAddr(host string) bool {
	return strings.HasPrefix(host, "/")
}

// JoinHostPort is like its equivalent in net., but handles UNIX sockets as well.
func JoinHostPort(host string, port int) string {
	if IsUnixAddr(host) {
		return host
	}

	return net.JoinHostPort(host, fmt.Sprint(port))
}

// ChanFromSlice takes a slice of values and returns a channel from which these values can be received.
// This channel is closed after the last value was sent.
func ChanFromSlice[T any](values []T) <-chan T {
	ch := make(chan T, len(values))
	for _, value := range values {
		ch <- value
	}

	close(ch)

	return ch
}

// PrintErrorThenExit prints the given error to [os.Stderr] and exits with the specified error code.
func PrintErrorThenExit(err error, exitCode int) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(exitCode)
}
