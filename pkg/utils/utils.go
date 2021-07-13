package utils

import (
	"context"
	"crypto/sha1"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/exp/utf8string"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"strings"
	"sync"
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

func Timed(start time.Time, callback func(elapsed time.Duration)) {
	callback(time.Since(start))
}

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

func BatchSliceOfInterfaces(ctx context.Context, keys []interface{}, count int) <-chan []interface{} {
	batches := make(chan []interface{})

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

func IsContextCanceled(err error) bool {
	return errors.Is(err, context.Canceled)
}

func CreateOrRead(name string, callback func() []byte) ([]byte, error) {
	info, err := os.Stat(name)

	if os.IsNotExist(err) {
		b := callback()
		if err := ioutil.WriteFile(name, b, 0660); err != nil {
			defer os.Remove(name)

			return nil, errors.Wrap(err, "can't write to file "+name)
		}

		return b, nil
	}

	if err != nil {
		return nil, errors.Wrap(err, "can't read file "+name)
	}

	if info.IsDir() {
		return nil, errors.Errorf(name + " is a directory")
	}

	b, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, errors.Wrap(err, "can't read file "+name)
	}

	return b, nil
}

func Uuid() []byte {
	u := uuid.New()

	return u[:]
}

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

func Fatal(err error) {
	// TODO(el): Print stacktrace via some recover() magic?
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

func RandomSleep(sugar *zap.SugaredLogger) {
	once := sync.Once{}
	once.Do(func() {
		rand.Seed(time.Now().UnixNano())
	})
	n := rand.Intn(100)
	d := time.Duration(n) * time.Millisecond
	sugar.Info("Sleeping for ", d)
	time.Sleep(d)
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

// Min returns the smaller of x or y.
func Min(x, y int) int {
	if x < y {
		return x
	}

	return y
}
