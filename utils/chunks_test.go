package utils

import (
	"github.com/magiconair/properties/assert"
	"testing"
)

func TestChunkKeys(t *testing.T) {
	keys := []string{
		"herp",
		"derp",
		"merp",
		"berp",
	}

	ch := ChunkKeys(make(chan struct{}), keys, 2)
	var chunks [][]string
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	want := [][]string{
		{
			"herp",
			"derp",
		},
		{
			"merp",
			"berp",
		},
	}

	assert.Equal(t, chunks, want)

	ch = ChunkKeys(make(chan struct{}), keys, 5)
	chunks = nil
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	want = [][]string{
		{
			"herp",
			"derp",
			"merp",
			"berp",
		},
	}

	assert.Equal(t, chunks, want)
}
