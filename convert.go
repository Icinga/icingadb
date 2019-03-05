package icingadb_utils

import "encoding/hex"

//Checksum converts a hex string to a byte array
func Checksum(s string) []byte {
	c, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}

	return c
}

