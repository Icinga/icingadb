package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEncodeChecksum(t *testing.T) {
	assert.Equal(t, []byte{
		218, 57, 163, 238, 94, 107, 75, 13, 50, 85, 191, 239, 149, 96, 24, 144, 175, 216, 7, 9,
	}, EncodeChecksum("da39a3ee5e6b4b0d3255bfef95601890afd80709"))

	assert.Panics(t, func() {
		EncodeChecksum("x")
	})
}

func TestDecodeChecksum(t *testing.T) {
	assert.Equal(t, "da39a3ee5e6b4b0d3255bfef95601890afd80709", DecodeChecksum([]byte{
		218, 57, 163, 238, 94, 107, 75, 13, 50, 85, 191, 239, 149, 96, 24, 144, 175, 216, 7, 9,
	}))
}

func TestChecksum(t *testing.T) {
	assert.Equal(t, "661295c9cbf9d6b2f6428414504a8deed3020641", Checksum("test string"))
	assert.Equal(t, "e6c8d9df653f15466261cc7c318743a7534f9462", Checksum("herp da derp"))
}

func TestNotificationTypesToBitMask(t *testing.T) {
	assert.Equal(t, 81, NotificationTypesToBitMask([]string{"DowntimeStart", "Acknowledgement", "Recovery"}))
	assert.Equal(t, 0, NotificationTypesToBitMask([]string{}))
}

func TestNotificationStatesToBitMask(t *testing.T) {
	assert.Equal(t, 53, NotificationStatesToBitMask([]string{"OK", "Up", "Down", "Critical"}))
	assert.Equal(t, 49, NotificationStatesToBitMask([]string{"OK", "Up", "Down"}))
}

func TestIcingaStateTypeToString(t *testing.T) {
	assert.Equal(t, "hard", IcingaStateTypeToString(0.0))
	assert.Equal(t, "soft", IcingaStateTypeToString(1.0))
}
