package types

import (
	"github.com/stretchr/testify/require"
	"testing"
	"unicode/utf8"
)

func TestBinary_MarshalJSON(t *testing.T) {
	subtests := []struct {
		name   string
		input  Binary
		output string
	}{
		{"nil", nil, `null`},
		{"empty", make(Binary, 0, 1), `null`},
		{"space", Binary(" "), `"20"`},
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
