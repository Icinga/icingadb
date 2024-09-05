package types

import (
	"database/sql/driver"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestStateType_UnmarshalJSON(t *testing.T) {
	subtests := []struct {
		name   string
		input  string
		output StateType
		error  bool
	}{
		{name: "bad-json", input: "bad JSON", error: true},
		{name: "bool", input: "false", error: true},
		{name: "string", input: `"0"`, error: true},
		{name: "negative", input: "-1", error: true},
		{name: "fraction", input: "0.5", error: true},
		{name: "out-of-range", input: "2", error: true},
		{name: "soft", input: "0", output: StateSoft},
		{name: "hard", input: "1", output: StateHard},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var s StateType
			if err := s.UnmarshalJSON([]byte(st.input)); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, st.output, s)
			}
		})
	}
}

func TestStateType_Value(t *testing.T) {
	subtests := []struct {
		name   string
		input  StateType
		output driver.Value
		error  bool
	}{
		{name: "invalid", input: 2, error: true},
		{name: "soft", input: StateSoft, output: "soft"},
		{name: "hard", input: StateHard, output: "hard"},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			if v, err := st.input.Value(); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, st.output, v)
			}
		})
	}
}
