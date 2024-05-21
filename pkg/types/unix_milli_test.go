package types

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
	"time"
	"unicode/utf8"
)

func TestUnixMilli(t *testing.T) {
	type testCase struct {
		v    UnixMilli
		json string
		text string
	}

	tests := map[string]testCase{
		"Zero":              {UnixMilli{}, "null", ""},
		"Non-zero":          {UnixMilli(time.Unix(1234567890, 0)), "1234567890000", "1234567890000"},
		"Epoch":             {UnixMilli(time.Unix(0, 0)), "0", "0"},
		"With milliseconds": {UnixMilli(time.Unix(1234567890, 62000000)), "1234567890062", "1234567890062"},
	}

	var runTests = func(t *testing.T, f func(*testing.T, testCase)) {
		for name, test := range tests {
			t.Run(name, func(t *testing.T) {
				f(t, test)
			})
		}
	}

	t.Run("MarshalJSON", func(t *testing.T) {
		runTests(t, func(t *testing.T, test testCase) {
			actual, err := test.v.MarshalJSON()
			require.NoError(t, err)
			require.True(t, utf8.Valid(actual))
			require.Equal(t, test.json, string(actual))
		})
	})

	t.Run("UnmarshalJSON", func(t *testing.T) {
		runTests(t, func(t *testing.T, test testCase) {
			var actual UnixMilli
			err := actual.UnmarshalJSON([]byte(test.json))
			require.NoError(t, err)
			require.Equal(t, test.v, actual)
		})
	})

	t.Run("MarshalText", func(t *testing.T) {
		runTests(t, func(t *testing.T, test testCase) {
			actual, err := test.v.MarshalText()
			require.NoError(t, err)
			require.True(t, utf8.Valid(actual))
			require.Equal(t, test.text, string(actual))
		})
	})

	t.Run("UnmarshalText", func(t *testing.T) {
		runTests(t, func(t *testing.T, test testCase) {
			var actual UnixMilli
			err := actual.UnmarshalText([]byte(test.text))
			require.NoError(t, err)
			require.Equal(t, test.v, actual)
		})
	})
}

func TestUnixMilli_Scan(t *testing.T) {
	tests := []struct {
		name      string
		v         any
		expected  UnixMilli
		expectErr bool
	}{
		{
			name:     "Nil",
			v:        nil,
			expected: UnixMilli{},
		},
		{
			name:     "Epoch",
			v:        int64(0),
			expected: UnixMilli(time.Unix(0, 0)),
		},
		{
			name:     "bytes",
			v:        []byte("1234567890062"),
			expected: UnixMilli(time.Unix(1234567890, 62000000)),
		},
		{
			name:      "Invalid bytes",
			v:         []byte("invalid"),
			expectErr: true,
		},
		{
			name:     "int64",
			v:        int64(1234567890062),
			expected: UnixMilli(time.Unix(1234567890, 62000000)),
		},
		{
			name:     "uint64",
			v:        uint64(1234567890062),
			expected: UnixMilli(time.Unix(1234567890, 62000000)),
		},
		{
			name:      "uint64 out of range for int64",
			v:         uint64(math.MaxInt64) + 1,
			expectErr: true,
		},
		{
			name:      "Invalid type",
			v:         "invalid",
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var actual UnixMilli
			err := actual.Scan(test.v)
			if test.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, actual)
			}
		})
	}
}

func TestUnixMilli_Value(t *testing.T) {
	t.Run("Zero", func(t *testing.T) {
		var zero UnixMilli
		actual, err := zero.Value()
		require.NoError(t, err)
		require.Nil(t, actual)
	})

	t.Run("Non-zero", func(t *testing.T) {
		withMilliseconds := time.Unix(1234567890, 62000000)
		expected := withMilliseconds.UnixMilli()
		actual, err := UnixMilli(withMilliseconds).Value()
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	})
}
