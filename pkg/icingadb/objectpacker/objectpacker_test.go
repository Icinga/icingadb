package objectpacker

import (
	"bytes"
	"github.com/icinga/icingadb/pkg/types"
	"io"
	"testing"
	"unsafe"
)

// limitedWriter allows writing a specific amount of data.
type limitedWriter struct {
	// limit specifies how many bytes to allow to write.
	limit int
}

var _ io.Writer = (*limitedWriter)(nil)

// Write returns io.EOF once lw.limit is exceeded, nil otherwise.
func (lw *limitedWriter) Write(p []byte) (n int, err error) {
	if len(p) <= lw.limit {
		lw.limit -= len(p)
		return len(p), nil
	}

	n = lw.limit
	err = io.EOF

	lw.limit = 0
	return
}

func TestLimitedWriter_Write(t *testing.T) {
	assertLimitedWriter_Write(t, 3, []byte{1, 2}, 2, nil, 1)
	assertLimitedWriter_Write(t, 3, []byte{1, 2, 3}, 3, nil, 0)
	assertLimitedWriter_Write(t, 3, []byte{1, 2, 3, 4}, 3, io.EOF, 0)
	assertLimitedWriter_Write(t, 0, []byte{1}, 0, io.EOF, 0)
	assertLimitedWriter_Write(t, 0, nil, 0, nil, 0)
}

func assertLimitedWriter_Write(t *testing.T, limitBefore int, p []byte, n int, err error, limitAfter int) {
	t.Helper()

	lw := limitedWriter{limitBefore}
	actualN, actualErr := lw.Write(p)

	if actualErr != err {
		t.Errorf("_, err := (&limitedWriter{%d}).Write(%#v); err != %#v", limitBefore, p, err)
	}

	if actualN != n {
		t.Errorf("n, _ := (&limitedWriter{%d}).Write(%#v); n != %d", limitBefore, p, n)
	}

	if lw.limit != limitAfter {
		t.Errorf("lw := limitedWriter{%d}; lw.Write(%#v); lw.limit != %d", limitBefore, p, limitAfter)
	}
}

func TestPackAny(t *testing.T) {
	assertPackAny(t, nil, []byte{0})
	assertPackAny(t, false, []byte{1})
	assertPackAny(t, true, []byte{2})

	assertPackAnyPanic(t, -42, 0)
	assertPackAnyPanic(t, int8(-42), 0)
	assertPackAnyPanic(t, int16(-42), 0)
	assertPackAnyPanic(t, int32(-42), 0)
	assertPackAnyPanic(t, int64(-42), 0)

	assertPackAnyPanic(t, uint(42), 0)
	assertPackAnyPanic(t, uint8(42), 0)
	assertPackAnyPanic(t, uint16(42), 0)
	assertPackAnyPanic(t, uint32(42), 0)
	assertPackAnyPanic(t, uint64(42), 0)
	assertPackAnyPanic(t, uintptr(42), 0)

	assertPackAnyPanic(t, float32(-42.5), 0)
	assertPackAny(t, -42.5, []byte{3, 0xc0, 0x45, 0x40, 0, 0, 0, 0, 0})

	assertPackAnyPanic(t, []struct{}(nil), 9)
	assertPackAnyPanic(t, []struct{}{}, 9)

	assertPackAny(t, []interface{}{nil, true, -42.5}, []byte{
		5, 0, 0, 0, 0, 0, 0, 0, 3,
		0,
		2,
		3, 0xc0, 0x45, 0x40, 0, 0, 0, 0, 0,
	})

	assertPackAny(t, []string{"", "a"}, []byte{
		5, 0, 0, 0, 0, 0, 0, 0, 2,
		4, 0, 0, 0, 0, 0, 0, 0, 0,
		4, 0, 0, 0, 0, 0, 0, 0, 1, 'a',
	})

	assertPackAnyPanic(t, []interface{}{0 + 0i}, 9)

	assertPackAnyPanic(t, map[struct{}]struct{}(nil), 9)
	assertPackAnyPanic(t, map[struct{}]struct{}{}, 9)

	assertPackAny(t, map[interface{}]interface{}{true: "", "nil": -42.5}, []byte{
		6, 0, 0, 0, 0, 0, 0, 0, 2,
		0, 0, 0, 0, 0, 0, 0, 3, 'n', 'i', 'l',
		3, 0xc0, 0x45, 0x40, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 4, 't', 'r', 'u', 'e',
		4, 0, 0, 0, 0, 0, 0, 0, 0,
	})

	assertPackAny(t, map[string]float64{"": 42}, []byte{
		6, 0, 0, 0, 0, 0, 0, 0, 1,
		0, 0, 0, 0, 0, 0, 0, 0,
		3, 0x40, 0x45, 0, 0, 0, 0, 0, 0,
	})

	assertPackAnyPanic(t, map[struct{}]struct{}{{}: {}}, 9)

	assertPackAnyPanic(t, (*int)(nil), 0)
	assertPackAny(t, new(float64), []byte{3, 0, 0, 0, 0, 0, 0, 0, 0})

	assertPackAny(t, "", []byte{4, 0, 0, 0, 0, 0, 0, 0, 0})
	assertPackAny(t, "a", []byte{4, 0, 0, 0, 0, 0, 0, 0, 1, 'a'})
	assertPackAny(t, "ä", []byte{4, 0, 0, 0, 0, 0, 0, 0, 2, 0xc3, 0xa4})

	{
		var binary [256]byte
		for i := range binary {
			binary[i] = byte(i)
		}

		assertPackAny(t, binary, append([]byte{4, 0, 0, 0, 0, 0, 0, 1, 0}, binary[:]...))
		assertPackAny(t, binary[:], append([]byte{4, 0, 0, 0, 0, 0, 0, 1, 0}, binary[:]...))
		assertPackAny(t, types.Binary(binary[:]), append([]byte{4, 0, 0, 0, 0, 0, 0, 1, 0}, binary[:]...))
	}

	{
		type myByte byte
		assertPackAnyPanic(t, []myByte(nil), 9)
	}

	assertPackAnyPanic(t, complex64(0+0i), 0)
	assertPackAnyPanic(t, 0+0i, 0)
	assertPackAnyPanic(t, make(chan struct{}, 0), 0)
	assertPackAnyPanic(t, func() {}, 0)
	assertPackAnyPanic(t, struct{}{}, 0)
	assertPackAnyPanic(t, unsafe.Pointer(uintptr(0)), 0)
}

func assertPackAny(t *testing.T, in interface{}, out []byte) {
	t.Helper()

	{
		buf := &bytes.Buffer{}
		if err := PackAny(in, buf); err == nil {
			if bytes.Compare(buf.Bytes(), out) != 0 {
				t.Errorf("buf := &bytes.Buffer{}; packAny(%#v, buf); bytes.Compare(buf.Bytes(), %#v) != 0", in, out)
			}
		} else {
			t.Errorf("packAny(%#v, &bytes.Buffer{}) != nil", in)
		}
	}

	for i := 0; i < len(out); i++ {
		if PackAny(in, &limitedWriter{i}) != io.EOF {
			t.Errorf("packAny(%#v, &limitedWriter{%d}) != io.EOF", in, i)
		}
	}
}

func assertPackAnyPanic(t *testing.T, in interface{}, allowToWrite int) {
	t.Helper()

	for i := 0; i < allowToWrite; i++ {
		if PackAny(in, &limitedWriter{i}) != io.EOF {
			t.Errorf("packAny(%#v, &limitedWriter{%d}) != io.EOF", in, i)
		}
	}

	defer func() {
		t.Helper()

		if r := recover(); r == nil {
			t.Errorf("packAny(%#v, &limitedWriter{%d}) didn't panic", in, allowToWrite)
		}
	}()

	_ = PackAny(in, &limitedWriter{allowToWrite})
}
