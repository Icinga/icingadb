package icingaredis

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

var latencies = []struct {
	name    string
	latency time.Duration
}{
	{"instantly", 0},
	{"1us", time.Microsecond},
	{"20ms", 20 * time.Millisecond},
}

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

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			for _, l := range latencies {
				t.Run(l.name, func(t *testing.T) {
					ctx := t.Context()

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

type testEntityWithChecksum struct {
	v1.EntityWithChecksum `json:",inline"`
	Data                  types.Binary `json:"data"`
}

func newTestEntityWithChecksum(id, checksum, data []byte) *testEntityWithChecksum {
	return &testEntityWithChecksum{
		EntityWithChecksum: v1.EntityWithChecksum{
			EntityWithoutChecksum: v1.EntityWithoutChecksum{IdMeta: v1.IdMeta{Id: id}},
			ChecksumMeta:          v1.ChecksumMeta{PropertiesChecksum: checksum},
		},
		Data: data,
	}
}

func newEntityWithChecksum(id, checksum []byte) *v1.EntityWithChecksum {
	return &v1.EntityWithChecksum{
		EntityWithoutChecksum: v1.EntityWithoutChecksum{IdMeta: v1.IdMeta{Id: id}},
		ChecksumMeta:          v1.ChecksumMeta{PropertiesChecksum: checksum},
	}
}

func TestSetChecksums(t *testing.T) {
	subtests := []struct {
		name      string
		input     []database.Entity
		checksums map[string]database.Entity
		output    []database.Entity
		error     bool
	}{
		{name: "nil"},
		{
			name:      "empty",
			checksums: map[string]database.Entity{},
		},
		{
			name:      "one",
			input:     []database.Entity{newTestEntityWithChecksum([]byte{1}, nil, []byte{3})},
			checksums: map[string]database.Entity{"01": newEntityWithChecksum([]byte{1}, []byte{2})},
			output:    []database.Entity{newTestEntityWithChecksum([]byte{1}, []byte{2}, []byte{3})},
		},
		{
			name: "two",
			input: []database.Entity{
				newTestEntityWithChecksum([]byte{4}, nil, []byte{6}),
				newTestEntityWithChecksum([]byte{7}, nil, []byte{9}),
			},
			checksums: map[string]database.Entity{
				"04": newEntityWithChecksum([]byte{4}, []byte{5}),
				"07": newEntityWithChecksum([]byte{7}, []byte{8}),
			},
			output: []database.Entity{
				newTestEntityWithChecksum([]byte{4}, []byte{5}, []byte{6}),
				newTestEntityWithChecksum([]byte{7}, []byte{8}, []byte{9}),
			},
		},
		{
			name: "three",
			input: []database.Entity{
				newTestEntityWithChecksum([]byte{10}, nil, []byte{12}),
				newTestEntityWithChecksum([]byte{13}, nil, []byte{15}),
				newTestEntityWithChecksum([]byte{16}, nil, []byte{18}),
			},
			checksums: map[string]database.Entity{
				"0a": newEntityWithChecksum([]byte{10}, []byte{11}),
				"0d": newEntityWithChecksum([]byte{13}, []byte{14}),
				"10": newEntityWithChecksum([]byte{16}, []byte{17}),
			},
			output: []database.Entity{
				newTestEntityWithChecksum([]byte{10}, []byte{11}, []byte{12}),
				newTestEntityWithChecksum([]byte{13}, []byte{14}, []byte{15}),
				newTestEntityWithChecksum([]byte{16}, []byte{17}, []byte{18}),
			},
		},
		{
			name:      "superfluous-checksum",
			checksums: map[string]database.Entity{"13": newEntityWithChecksum([]byte{19}, []byte{20})},
		},
		{
			name:  "missing-checksum",
			input: []database.Entity{newTestEntityWithChecksum([]byte{22}, nil, []byte{24})},
			error: true,
		},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			for _, concurrency := range []int{1, 2, 30} {
				t.Run(fmt.Sprint(concurrency), func(t *testing.T) {
					for _, l := range latencies {
						t.Run(l.name, func(t *testing.T) {
							ctx, cancel := context.WithCancel(context.Background())
							defer cancel()

							input := make(chan database.Entity, 1)
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
									case input <- v:
									case <-ctx.Done():
										return
									}
								}
							}()

							output, errs := SetChecksums(ctx, input, st.checksums, concurrency)

							require.NotNil(t, output, "output channel should not be nil")
							require.NotNil(t, errs, "error channel should not be nil")

							for _, expected := range st.output {
								select {
								case actual, ok := <-output:
									require.True(t, ok, "output channel should not be closed, yet")
									if concurrency == 1 || l.latency >= time.Millisecond {
										require.Equal(t, expected, actual)
									}
								case <-time.After(time.Second):
									require.Fail(t, "output channel should not block")
								}
							}

							if st.error {
								select {
								case err, ok := <-errs:
									require.True(t, ok, "error channel should not be closed, yet")
									require.Error(t, err)
								case <-time.After(time.Second):
									require.Fail(t, "error channel should not block")
								}
							}

							select {
							case actual, ok := <-output:
								require.False(t, ok, "output channel should be closed, got %#v", actual)
							case <-time.After(time.Second):
								require.Fail(t, "output channel should not block")
							}

							select {
							case err, ok := <-errs:
								require.False(t, ok, "error channel should be closed, got %#v", err)
							case <-time.After(time.Second):
								require.Fail(t, "error channel should not block")
							}
						})
					}
				})
			}

			for _, concurrency := range []int{0, -1, -2, -30} {
				t.Run(fmt.Sprint(concurrency), func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					input := make(chan database.Entity, 1)
					input <- nil

					output, errs := SetChecksums(ctx, input, st.checksums, concurrency)

					require.NotNil(t, output, "output channel should not be nil")
					require.NotNil(t, errs, "error channel should not be nil")

					select {
					case v, ok := <-output:
						require.False(t, ok, "output channel should be closed, got %#v", v)
					case <-time.After(time.Second):
						require.Fail(t, "output channel should not block")
					}

					select {
					case err, ok := <-errs:
						require.False(t, ok, "error channel should be closed, got %#v", err)
					case <-time.After(time.Second):
						require.Fail(t, "error channel should not block")
					}

					select {
					case input <- nil:
						require.Fail(t, "input channel should not be read from")
					default:
					}
				})
			}
		})
	}

	t.Run("cancel-ctx", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		output, errs := SetChecksums(ctx, make(chan database.Entity), map[string]database.Entity{}, 1)

		require.NotNil(t, output, "output channel should not be nil")
		require.NotNil(t, errs, "error channel should not be nil")

		select {
		case v, ok := <-output:
			require.False(t, ok, "output channel should be closed, got %#v", v)
		case <-time.After(time.Second):
			require.Fail(t, "output channel should not block")
		}

		select {
		case err, ok := <-errs:
			require.True(t, ok, "error channel should not be closed, yet")
			require.Error(t, err)
		case <-time.After(time.Second):
			require.Fail(t, "error channel should not block")
		}
	})
}
