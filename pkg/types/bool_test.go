package types

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
	"unicode/utf8"
)

func TestBool_MarshalJSON(t *testing.T) {
	subtests := []struct {
		input  Bool
		output string
	}{
		{Bool{Bool: false, Valid: false}, `null`},
		{Bool{Bool: false, Valid: true}, `false`},
		{Bool{Bool: true, Valid: false}, `null`},
		{Bool{Bool: true, Valid: true}, `true`},
	}

	for _, st := range subtests {
		t.Run(fmt.Sprintf("Bool-%#v_Valid-%#v", st.input.Bool, st.input.Valid), func(t *testing.T) {
			actual, err := st.input.MarshalJSON()

			require.NoError(t, err)
			require.True(t, utf8.Valid(actual))
			require.Equal(t, st.output, string(actual))
		})
	}
}
