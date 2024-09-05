package types

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAcknowledgementState_UnmarshalJSON(t *testing.T) {
	subtests := []struct {
		name   string
		input  string
		output AcknowledgementState
		error  bool
	}{
		{name: "bad-json", input: "bad JSON", error: true},
		{name: "bool", input: "true", error: true},
		{name: "string", input: `"1"`, error: true},
		{name: "negative", input: "-1", error: true},
		{name: "fraction", input: "1.5", error: true},
		{name: "out-of-range", input: "3", error: true},
		{name: "n", input: "0", output: 0},
		{name: "y", input: "1", output: 1},
		{name: "sticky", input: "2", output: 2},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var a AcknowledgementState
			if err := a.UnmarshalJSON([]byte(st.input)); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, st.output, a)
			}
		})
	}
}
