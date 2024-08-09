package icingaredis

import (
	"context"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type testEntity struct {
	v1.EntityWithoutChecksum `json:",inline"`
	Data                     int `json:"data"`
}

func newTestEntity(data int, id ...byte) *testEntity {
	return &testEntity{
		EntityWithoutChecksum: v1.EntityWithoutChecksum{
			IdMeta: v1.IdMeta{Id: id},
		},
		Data: data,
	}
}

func factoryFunc() database.Entity {
	return &testEntity{}
}

func TestCreateEntities(t *testing.T) {
	subtests := []struct {
		name       string
		pairs      []redis.HPair
		concurrent int
		error      bool
		output     []database.Entity
	}{
		{"empty", nil, 1, false, nil},
		{"kerr", []redis.HPair{{Field: "172", Value: `{"data":1337}`}}, 1, true, nil},
		{"verr", []redis.HPair{{Field: "172a", Value: `{"data":1337`}}, 1, true, nil},
		{"one", []redis.HPair{{Field: "172a", Value: `{"data":1337}`}}, 1, false, []database.Entity{
			newTestEntity(1337, 23, 42),
		}},
		{
			"two", []redis.HPair{{Field: "17", Value: `{"data":2038}`}, {Field: "2a", Value: `{"data":777}`}}, 1, false,
			[]database.Entity{newTestEntity(2038, 23), newTestEntity(777, 42)},
		},
		{"concurrent", []redis.HPair{{Field: "172a", Value: `{"data":1337}`}}, 10, false, []database.Entity{
			newTestEntity(1337, 23, 42),
		}},
		{
			"ok_and_err", []redis.HPair{
				{Field: "17", Value: `{"data":2038}`},
				{Field: "172", Value: `{"data":1337}`},
				{Field: "2a", Value: `{"data":777}`},
			}, 1, true, []database.Entity{
				newTestEntity(2038, 23),
			},
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

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			for _, l := range latencies {
				t.Run(l.name, func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					input := make(chan redis.HPair, 1)
					go func() {
						defer close(input)

						for _, v := range st.pairs {
							if l.latency > 0 {
								select {
								case <-time.After(l.latency):
								case <-ctx.Done():
									return
								}
							}

							select {
							case input <- v:
							case <-ctx.Done():
								return
							}
						}
					}()

					entities, errors := CreateEntities(ctx, factoryFunc, input, st.concurrent)

					require.NotNil(t, entities, "entities channel should not be nil")
					require.NotNil(t, errors, "errors channel should not be nil")

					for i, expected := range st.output {
						errs := errors
						if st.error && i == len(st.output)-1 {
							errs = nil
						}

						select {
						case actual, ok := <-entities:
							require.True(t, ok, "entities channel should not be closed, yet")
							require.Equal(t, expected, actual)
						case <-errs:
							require.Fail(t, "errors channel should not be ready (yet)")
						case <-time.After(time.Second):
							require.Fail(t, "entities channel should not block")
						}
					}

					if st.error {
						select {
						case err, ok := <-errors:
							require.True(t, ok, "errors channel should not be closed, yet")
							require.Error(t, err)
						case <-time.After(time.Second):
							require.Fail(t, "errors channel should not block")
						}
					}

					select {
					case _, ok := <-entities:
						require.False(t, ok, "entities channel should be closed")
					case <-time.After(time.Second):
						require.Fail(t, "entities channel should not block")
					}

					select {
					case _, ok := <-errors:
						require.False(t, ok, "errors channel should be closed")
					case <-time.After(time.Second):
						require.Fail(t, "errors channel should not block")
					}
				})
			}
		})
	}
}
