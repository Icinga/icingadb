package types

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNotificationTypes_UnmarshalJSON(t *testing.T) {
	subtests := []struct {
		name   string
		input  string
		output NotificationTypes
		error  bool
	}{
		{name: "bad-json", input: "bad JSON", error: true},
		{name: "string", input: `"Problem"`, error: true},
		{name: "bad-type", input: `["bad type"]`, error: true},
		{name: "empty", input: `[]`, output: 0},
		{name: "single", input: `["Problem"]`, output: 32},
		{name: "multiple", input: `["Problem", "Acknowledgement"]`, output: 48},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var s NotificationTypes
			if err := s.UnmarshalJSON([]byte(st.input)); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, st.output, s)
			}
		})
	}
}
