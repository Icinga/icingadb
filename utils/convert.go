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

/**
 * Convert the given []byte to a [20]byte
 *
 * @param  in  []byte  Input
 *
 * @returns  out  [20]byte  Output
 */
func Bytes2checksum(in []byte) (out [20]byte) {
	copy(out[:], in)
	return
}

/**
 * Convert the given [20]byte to a []byte
 *
 * @param  in  [20]byte  Input
 *
 * @returns  out  []byte  Output
 */
func Checksum2bytes(in [20]byte) (out []byte) {
	out = make([]byte, 20)
	copy(out, in[:])
	return
}


func DecodeChecksum(c []byte) string {
	return hex.EncodeToString(c)
}