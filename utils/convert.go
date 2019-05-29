package utils

import (
	"encoding/hex"
)

var (
	Bool = map[bool]string{
		true:  "y",
		false: "n",
	}
)

// Checksum converts a hex string to a byte array
func Checksum(s string) []byte {
	c, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}

	return c
}

// DecodeChecksum coverts a byte array into a hex string
func DecodeChecksum(c []byte) string {
	return hex.EncodeToString(c)
}