package utils

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/icinga/icingadb/pkg/contracts"
	"go.uber.org/zap"
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

func SyncMapKeys(m *sync.Map) []string {
	keys := make([]string, 0)

	if m != nil {
		m.Range(func(key, value interface{}) bool {
			keys = append(keys, key.(string))

			return true
		})
	}

	return keys
}

func SyncMapIDs(m *sync.Map) []interface{} {
	ids := make([]interface{}, 0)

	if m != nil {
		m.Range(func(key, value interface{}) bool {
			ids = append(ids, value.(contracts.IDer).ID())

			return true
		})
	}

	return ids
}

func SyncMapEntities(m *sync.Map) <-chan contracts.Entity {
	entities := make(chan contracts.Entity, 0)

	go func() {
		defer close(entities)
		if m != nil {
			m.Range(func(key, value interface{}) bool {
				entities <- value.(contracts.Entity)

				return true
			})
		}

	}()

	return entities
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

			return nil, err
		}

		return b, nil
	}

	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return nil, fmt.Errorf("'%s' is a directory", name)
	}

	b, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, err
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
	switch e := err.(type) {
	case *mysql.MySQLError:
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
