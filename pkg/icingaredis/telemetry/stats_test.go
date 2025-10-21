package telemetry

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStatsKeeper(t *testing.T) {
	desiredState := map[string]uint64{
		"foo": 23,
		"bar": 42,
		"baz": 0,
	}

	stats := &StatsKeeper{}

	// Populate based on desiredState
	for key, counterValue := range desiredState {
		ctr := stats.Get(key)
		ctr.Add(counterValue)
	}

	// Check if desiredState is set
	for key, counterValue := range desiredState {
		ctr := stats.Get(key)
		assert.Equal(t, counterValue, ctr.Val())
	}

	// Get reference, change value, compare
	fooKey := "foo"
	fooCtr := stats.Get(fooKey)
	assert.Equal(t, desiredState[fooKey], fooCtr.Reset())
	assert.Equal(t, uint64(0), fooCtr.Val())
	assert.Equal(t, uint64(0), stats.Get(fooKey).Val())
	fooCtr.Add(desiredState[fooKey])
	assert.Equal(t, desiredState[fooKey], stats.Get(fooKey).Val())

	// Range over
	for key, ctr := range stats.Iterator() {
		desired, ok := desiredState[key]
		assert.True(t, ok)
		assert.Equal(t, desired, ctr.Val())
	}
}
