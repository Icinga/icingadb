package types

import (
	"database/sql/driver"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCommentType_UnmarshalJSON(t *testing.T) {
	subtests := []struct {
		name   string
		input  string
		output CommentType
		error  bool
	}{
		{name: "bad-json", input: "bad JSON", error: true},
		{name: "bool", input: "true", error: true},
		{name: "string", input: `"1"`, error: true},
		{name: "negative", input: "-1", error: true},
		{name: "fraction", input: "1.5", error: true},
		{name: "out-of-range", input: "5", error: true},
		{name: "comment", input: "1", output: 1},
		{name: "ack", input: "4", output: 4},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			var c CommentType
			if err := c.UnmarshalJSON([]byte(st.input)); st.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, st.output, c)
			}
		})
	}
}

func TestCommentType_Value(t *testing.T) {
	subtests := []struct {
		name   string
		input  CommentType
		output driver.Value
		error  bool
	}{
		{name: "invalid", input: 5, error: true},
		{name: "comment", input: 1, output: "comment"},
		{name: "ack", input: 4, output: "ack"},
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
