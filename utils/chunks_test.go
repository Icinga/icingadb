// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

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

func TestChunkInterfaces(t *testing.T) {
	interfaces := []interface{}{
		"herp",
		"derp",
		"merp",
		"berp",
	}

	chunks := ChunkInterfaces(interfaces, 2)
	want := [][]interface{}{
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

	chunks = ChunkInterfaces(interfaces, 5)
	want = [][]interface{}{
		{
			"herp",
			"derp",
			"merp",
			"berp",
		},
	}

	assert.Equal(t, want, chunks)
}

func TestTestChunkIndices(t *testing.T) {
	assert.Equal(t, 0, len(ChunkIndices(0, 1)), "chunking zero elements should return zero chunks")

	assert.Equal(t, []Chunk{
		{0, 10},
		{10, 20},
	}, ChunkIndices(20, 10), "chunking 20 elements into chunks of size 10")

	assert.Equal(t, []Chunk{
		{0, 23},
	}, ChunkIndices(23, 42), "chunking 23 elements into chunks of size 42")

	assert.Equal(t, []Chunk{
		{0, 23},
		{23, 42},
	}, ChunkIndices(42, 23), "chunking 42 elements into chunks of size 23")
}
