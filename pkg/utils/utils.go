package utils

import (
	"context"
	"crypto/sha1"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/icinga/icingadb/pkg/database"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"golang.org/x/exp/utf8string"
	"math"
	"net"
	"os"
	"path/filepath"
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
	if tn, ok := t.(database.TableNamer); ok {
		return tn.TableName()
	} else {
		return Key(Name(t), '_')
	}
}

// Key returns the name with all Unicode letters mapped to lower case letters,
// with an additional separator in front of each original upper case letter.
func Key(name string, sep byte) string {
	return ConvertCamelCase(name, unicode.LowerCase, sep)
}

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

// ConvertCamelCase converts a (lower) CamelCase string into various cases.
// _case must be unicode.Lower or unicode.Upper.
//
// Example usage:
//
//	# snake_case
//	ConvertCamelCase(s, unicode.Lower, '_')
//
//	# SCREAMING_SNAKE_CASE
//	ConvertCamelCase(s, unicode.Upper, '_')
//
//	# kebab-case
//	ConvertCamelCase(s, unicode.Lower, '-')
//
//	# SCREAMING-KEBAB-CASE
//	ConvertCamelCase(s, unicode.Upper, '-')
//
//	# other.separator
//	ConvertCamelCase(s, unicode.Lower, '.')
func ConvertCamelCase(s string, _case int, sep byte) string {
	r := []rune(s)
	b := strings.Builder{}
	b.Grow(len(r) + 2) // nominal 2 bytes of extra space for inserted delimiters

	b.WriteRune(unicode.To(_case, r[0]))
	for _, r := range r[1:] {
		if sep != 0 && unicode.IsUpper(r) {
			b.WriteByte(sep)
		}

		b.WriteRune(unicode.To(_case, r))
	}

	return b.String()
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

// JoinHostPort is like its equivalent in net., but handles UNIX sockets as well.
func JoinHostPort(host string, port int) string {
	if strings.HasPrefix(host, "/") {
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
