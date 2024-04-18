package types

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"unicode/utf8"
)

func TestUnixNano_MarshalJSON(t *testing.T) {
	subtests := []struct {
		name   string
		input  UnixNano
		output string
	}{
		{"zero", UnixNano{}, `null`},
		{"epoch", UnixNano(time.Unix(0, 0)), `0`},
		{"nonzero", UnixNano(time.Unix(1234567890, 62500000)), `1234567890062500000`},
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
