package utils

import (
	"github.com/stretchr/testify/assert"
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

	assert.ElementsMatch(t, []string{
		"cherp",
		"lerp",
	}, introduced)

	assert.ElementsMatch(t, []string{
		"herp",
		"merp",
	}, maintained)

	assert.ElementsMatch(t, []string{
		"derp",
		"berp",
	}, dismissed)
}
