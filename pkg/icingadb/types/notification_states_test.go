package types

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNotificationStates_UnmarshalJSON(t *testing.T) {
	subtests := []struct {
		name   string
		input  string
		output NotificationStates
		error  bool
	}{
		{name: "bad-json", input: "bad JSON", error: true},
		{name: "string", input: `"OK"`, error: true},
		{name: "bad-state", input: `["bad state"]`, error: true},
		{name: "empty", input: `[]`, output: 0},
		{name: "single", input: `["OK"]`, output: 1},
		{name: "multiple", input: `["OK", "Warning"]`, output: 3},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var s NotificationStates
			if err := s.UnmarshalJSON([]byte(st.input)); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, st.output, s)
			}
		})
	}
}

func TestNotificationStates_Value(t *testing.T) {
	subtests := []struct {
		name  string
		input NotificationStates
		error bool
	}{
		{name: "out-of-range", input: ^NotificationStates(0), error: true},
		{name: "empty", input: 0},
		{name: "single", input: 4},
		{name: "multiple", input: 7},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			if v, err := st.input.Value(); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, int64(st.input), v)
			}
		})
	}
}
