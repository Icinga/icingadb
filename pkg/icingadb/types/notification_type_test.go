package types

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNotificationType_UnmarshalText(t *testing.T) {
	subtests := []struct {
		name   string
		input  string
		output NotificationType
		error  bool
	}{
		{name: "not-number", input: "not a number", error: true},
		{name: "negative", input: "-1", error: true},
		{name: "fraction", input: "1.5", error: true},
		{name: "out-of-range", input: "1000000", error: true},
		{name: "none", input: "0", error: true},
		{name: "multiple", input: "48", error: true},
		{name: "single", input: "32", output: 32},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var s NotificationType
			if err := s.UnmarshalText([]byte(st.input)); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, st.output, s)
			}
		})
	}
}
