package icingadb

import (
	"context"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
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

func newEntity(id []byte) *v1.EntityWithoutChecksum {
	return &v1.EntityWithoutChecksum{IdMeta: v1.IdMeta{Id: id}}
}

func TestEntitiesById_IDs(t *testing.T) {
	subtests := []struct {
		name   string
		input  EntitiesById
		output []types.Binary
	}{
		{name: "nil"},
		{
			name:  "empty",
			input: EntitiesById{},
		},
		{
			name:   "one",
			input:  EntitiesById{"one": newEntity([]byte{23})},
			output: []types.Binary{{23}},
		},
		{
			name:   "two",
			input:  EntitiesById{"one": newEntity([]byte{23}), "two": newEntity([]byte{42})},
			output: []types.Binary{{23}, {42}},
		},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			require.ElementsMatch(t, st.output, st.input.IDs())
		})
	}
}

func TestEntitiesById_Entities(t *testing.T) {
	subtests := []struct {
		name string
		io   EntitiesById
	}{
		{name: "nil"},
		{
			name: "empty",
			io:   EntitiesById{},
		},
		{
			name: "one",
			io:   EntitiesById{"one": newEntity([]byte{23})},
		},
		{
			name: "two",
			io:   EntitiesById{"one": newEntity([]byte{23}), "two": newEntity([]byte{42})},
		},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			ctx := t.Context()

			expected := make([]database.Entity, 0, len(st.io))
			actual := make([]database.Entity, 0, len(st.io))

			for _, v := range st.io {
				expected = append(expected, v)
			}

			ch := st.io.Entities(ctx)
			require.NotNil(t, ch)

			for range expected {
				select {
				case v, ok := <-ch:
					require.True(t, ok, "channel closed too early")
					actual = append(actual, v)
				case <-time.After(time.Second):
					require.Fail(t, "channel should not block")
				}
			}

			require.ElementsMatch(t, expected, actual)

			select {
			case v, ok := <-ch:
				require.False(t, ok, "channel should be closed, got %#v", v)
			case <-time.After(time.Second):
				require.Fail(t, "channel should not block")
			}
		})
	}

	t.Run("closed-ctx", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		ch := EntitiesById{"one": newEntity([]byte{23})}.Entities(ctx)
		require.NotNil(t, ch)

		time.Sleep(time.Millisecond)

		select {
		case v, ok := <-ch:
			require.False(t, ok, "channel should be closed, got %#v", v)
		case <-time.After(time.Second):
			require.Fail(t, "channel should not block")
		}
	})
}
