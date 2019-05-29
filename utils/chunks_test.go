package utils

import (
	"github.com/stretchr/testify/assert"
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

	assert.Equal(t, want, chunks)

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

	assert.Equal(t, want, chunks)
}
