package utils

import (
	"github.com/magiconair/properties/assert"
	"testing"
)

func TestDelta(t *testing.T) {
	old := []string{
		"herp",
		"merp",
		"derp",
		"berp",
	}

	new := []string{
		"herp",
		"merp",
		"cherp",
		"lerp",
	}

	introduced, maintained, dismissed := Delta(new, old)

	assert.Equal(t, []string{
		"cherp",
		"lerp",
	}, introduced)

	assert.Equal(t, []string{
		"herp",
		"merp",
	}, maintained)

	assert.Equal(t, []string{
		"derp",
		"berp",
	}, dismissed)
}
