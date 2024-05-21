package types

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"unicode/utf8"
)

func TestUnixMilli_MarshalJSON(t *testing.T) {
	subtests := []struct {
		name   string
		input  UnixMilli
		output string
	}{
		{"zero", UnixMilli{}, `null`},
		{"epoch", UnixMilli(time.Unix(0, 0)), `0`},
		{"nonzero", UnixMilli(time.Unix(1234567890, 62500000)), `1234567890062`},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			actual, err := st.input.MarshalJSON()

			require.NoError(t, err)
			require.True(t, utf8.Valid(actual))
			require.Equal(t, st.output, string(actual))
		})
	}
}

func TestUnixMilli_UnmarshalText(t *testing.T) {
	subtests := []struct {
		name   string
		input  string
		output UnixMilli
	}{
		{"zero", "", UnixMilli{}},
		{"epoch", "0", UnixMilli(time.Unix(0, 0))},
		{"nonzero", "1234567890062", UnixMilli(time.Unix(1234567890, 62000000))},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var actual UnixMilli
			err := actual.UnmarshalText([]byte(st.input))

			require.NoError(t, err)
			require.Equal(t, st.output, actual)
		})
	}
}
