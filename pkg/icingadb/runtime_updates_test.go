package icingadb

import (
	"context"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icinga-go-library/structify"
	"github.com/icinga/icinga-go-library/types"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"reflect"
	"testing"
	"time"
)

func makeChannel[T any]() chan T {
	return make(chan T)
}

func makeNilChannel[T any]() chan T {
	return nil
}

func makeClosedChannel[T any]() chan T {
	ch := make(chan T)
	close(ch)
	return ch
}

func newEntity(id ...byte) database.Entity {
	return &v1.EntityWithoutChecksum{IdMeta: v1.IdMeta{Id: id}}
}

func TestStructifyStream(t *testing.T) {
	type output struct {
		upsertEntities []database.Entity
		deleteIds      []types.Binary
	}

	structifier := structify.MakeMapStructifier(reflect.TypeOf((*v1.EntityWithoutChecksum)(nil)).Elem(), "json", nil)

	subtests := []struct {
		name   string
		input  []map[string]any
		output []output
		error  bool
	}{
		{name: "none"},
		{
			name:  "cant-structify",
			input: []map[string]any{{"runtime_type": "upsert", "id": "not hex"}},
			error: true,
		},
		{
			name:  "missing-runtime-type",
			input: []map[string]any{nil},
			error: true,
		},
		{
			name:  "invalid-runtime-type",
			input: []map[string]any{{"runtime_type": "invalid"}},
			error: true,
		},
		{
			name:   "empty-upsert",
			input:  []map[string]any{{"runtime_type": "upsert"}},
			output: []output{{upsertEntities: []database.Entity{&v1.EntityWithoutChecksum{}}}},
		},
		{
			name:   "empty-delete",
			input:  []map[string]any{{"runtime_type": "delete"}},
			output: []output{{deleteIds: []types.Binary{nil}}},
		},
		{
			name:   "upsert",
			input:  []map[string]any{{"runtime_type": "upsert", "id": "23"}},
			output: []output{{upsertEntities: []database.Entity{newEntity(0x23)}}},
		},
		{
			name:   "delete",
			input:  []map[string]any{{"runtime_type": "delete", "id": "42"}},
			output: []output{{deleteIds: []types.Binary{{0x42}}}},
		},
		{
			name:   "mixed",
			input:  []map[string]any{{"runtime_type": "upsert", "id": "23"}, {"runtime_type": "delete", "id": "42"}},
			output: []output{{upsertEntities: []database.Entity{newEntity(0x23)}}, {deleteIds: []types.Binary{{0x42}}}},
		},
	}

	latencies := []struct {
		name    string
		latency time.Duration
	}{
		{"instantly", 0},
		{"1us", time.Microsecond},
		{"20ms", 20 * time.Millisecond},
	}

	delays := []struct {
		name            string
		block           bool
		upsertedFactory func() chan database.Entity
		deletedFactory  func() chan any
	}{
		{"delay", true, makeChannel[database.Entity], makeChannel[any]},
		{"nodelay-nil", false, makeNilChannel[database.Entity], makeNilChannel[any]},
		{"nodelay-closed", false, makeClosedChannel[database.Entity], makeClosedChannel[any]},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			for _, l := range latencies {
				t.Run(l.name, func(t *testing.T) {
					for _, d := range delays {
						t.Run(d.name, func(t *testing.T) {
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()

							input := make(chan redis.XMessage, 1)
							go func() {
								defer close(input)

								for _, v := range st.input {
									if l.latency > 0 {
										select {
										case <-time.After(l.latency):
										case <-ctx.Done():
											return
										}
									}

									select {
									case input <- redis.XMessage{Values: v}:
									case <-ctx.Done():
										return
									}
								}
							}()

							upsertEntities := make(chan database.Entity)
							upserted := d.upsertedFactory()
							deleteIds := make(chan any)
							deleted := d.deletedFactory()

							f := structifyStream(ctx, input, upsertEntities, upserted, deleteIds, deleted, structifier)
							require.NotNil(t, f, "output function should not be nil")

							g, _ := errgroup.WithContext(ctx)
							g.Go(f)
							err := com.WaitAsync(g)

							for _, output := range st.output {
								for _, expected := range output.upsertEntities {
									select {
									case actual, ok := <-upsertEntities:
										require.True(t, ok, "upsertEntities channel should not be closed, yet")
										require.Equal(t, expected, actual)
									case <-time.After(time.Second):
										require.Fail(t, "upsertEntities channel should not block")
									}

									if d.block {
										select {
										case actual, ok := <-upsertEntities:
											require.True(t, ok, "upsertEntities channel should not be closed, yet")
											require.Fail(t, "upsertEntities channel should block, got %#v", actual)
										case actual, ok := <-deleteIds:
											require.True(t, ok, "deleteIds channel should not be closed, yet")
											require.Fail(t, "deleteIds channel should block, got %#v", actual)
										case deleted <- nil:
											require.Fail(t, "deleted channel should block")
										case e := <-err:
											require.Fail(t, "unexpected error: %#v", e)
										case <-time.After(time.Second / 10):
										}

										select {
										case upserted <- nil:
										case <-time.After(time.Second):
											require.Fail(t, "upserted channel should not block")
										}
									}
								}

								for _, expected := range output.deleteIds {
									select {
									case actual, ok := <-deleteIds:
										require.True(t, ok, "deleteIds channel should not be closed, yet")
										require.Equal(t, expected, actual)
									case <-time.After(time.Second):
										require.Fail(t, "deleteIds channel should not block")
									}

									if d.block {
										select {
										case actual, ok := <-upsertEntities:
											require.True(t, ok, "upsertEntities channel should not be closed, yet")
											require.Fail(t, "upsertEntities channel should block, got %#v", actual)
										case actual, ok := <-deleteIds:
											require.True(t, ok, "deleteIds channel should not be closed, yet")
											require.Fail(t, "deleteIds channel should block, got %#v", actual)
										case upserted <- nil:
											require.Fail(t, "upserted channel should block")
										case e := <-err:
											require.Fail(t, "unexpected error: %#v", e)
										case <-time.After(time.Second / 10):
										}

										select {
										case deleted <- nil:
										case <-time.After(time.Second):
											require.Fail(t, "deleted channel should not block")
										}
									}
								}
							}

							select {
							case e := <-err:
								if st.error {
									require.Error(t, e)
								} else {
									require.NoError(t, e)
								}
							case <-time.After(time.Second):
								require.Fail(t, "function should return in time")
							}

							select {
							case actual, ok := <-upsertEntities:
								require.False(t, ok, "upsertEntities channel should be closed, got %#v", actual)
							case <-time.After(time.Second):
								require.Fail(t, "upsertEntities channel should not block")
							}

							select {
							case actual, ok := <-deleteIds:
								require.False(t, ok, "deleteIds channel should be closed, got %#v", actual)
							case <-time.After(time.Second):
								require.Fail(t, "deleteIds channel should not block")
							}
						})
					}
				})
			}
		})
	}
}
