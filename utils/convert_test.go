package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestChecksum(t *testing.T) {
	assert.Equal(t, []byte{
		218, 57, 163, 238, 94, 107, 75, 13, 50, 85, 191, 239, 149, 96, 24, 144, 175, 216, 7, 9,
	}, Checksum("da39a3ee5e6b4b0d3255bfef95601890afd80709"))

	assert.Panics(t, func() {
		Checksum("x")
	})
}

func TestDecodeChecksum(t *testing.T) {
	assert.Equal(t, "da39a3ee5e6b4b0d3255bfef95601890afd80709", DecodeChecksum([]byte{
		218, 57, 163, 238, 94, 107, 75, 13, 50, 85, 191, 239, 149, 96, 24, 144, 175, 216, 7, 9,
	}))
}
