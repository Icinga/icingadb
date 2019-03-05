package icingadb_utils

import (
	"crypto/sha1"
	"encoding/hex"
)

var (
	Bool = map[bool]string{
		true:  "y",
		false: "n",
	}
)

//Checksum converts a hex string to a byte array
func Checksum(s string) []byte {
	c, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}

	return c
}

func Sha1(s string) []byte {
	h := sha1.New()
	h.Write([]byte(s))
	bs := h.Sum(nil)

	return bs
}

func DecodeChecksum(c []byte) string {
	return hex.EncodeToString(c)
}