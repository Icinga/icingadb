package icingadb

import (
	"github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEntitiesById_Keys(t *testing.T) {
	subtests := []struct {
		name   string
		input  EntitiesById
		output []string
	}{
		{name: "nil"},
		{
			name:  "empty",
			input: EntitiesById{},
		},
		{
			name:   "one",
			input:  EntitiesById{"one": nil},
			output: []string{"one"},
		},
		{
			name:   "two",
			input:  EntitiesById{"one": nil, "two": &v1.EntityWithoutChecksum{}},
			output: []string{"one", "two"},
		},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			require.ElementsMatch(t, st.output, st.input.Keys())
		})
	}
}
