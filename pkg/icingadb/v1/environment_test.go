package v1

import (
	"context"
	"github.com/icinga/icinga-go-library/types"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestEnvironment_NewContext(t *testing.T) {
	deadline := time.Now().Add(time.Minute)
	parent, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	actual, ok := (*Environment)(nil).NewContext(parent).Deadline()

	require.True(t, ok)
	require.Equal(t, deadline, actual)
}

func TestEnvironmentFromContext(t *testing.T) {
	subtests := []struct {
		name   string
		input  context.Context
		output *Environment
		ok     bool
	}{
		{
			name:  "background",
			input: context.Background(),
		},
		{
			name:  "nil",
			input: (*Environment)(nil).NewContext(context.Background()),
			ok:    true,
		},
		{
			name:   "empty",
			input:  (&Environment{}).NewContext(context.Background()),
			output: &Environment{},
			ok:     true,
		},
		{
			name:   "named",
			input:  (&Environment{Name: types.MakeString("foobar")}).NewContext(context.Background()),
			output: &Environment{Name: types.MakeString("foobar")},
			ok:     true,
		},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			actual, ok := EnvironmentFromContext(st.input)

			require.Equal(t, st.output, actual)
			require.Equal(t, st.ok, ok)
		})
	}
}
