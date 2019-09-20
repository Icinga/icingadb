package utils

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
)

var (
	Bool = map[bool]string{
		true:  "y",
		false: "n",
	}
	NotificationStates = map[string]int{
		"OK": 		1,
		"Warning":	2,
		"Critical": 4,
		"Unknown":	8,
		"Up":		16,
		"Down":		32,
	}
	NotificationTypes = map[string]int{
		"DowntimeStart": 	1,
		"DowntimeEnd":		2,
		"DowntimeRemoved": 	4,
		"Custom":			8,
		"Acknowledgement":	16,
		"Problem":			32,
		"Recovery":			64,
		"FlappingStart":	128,
		"FlappingEnd":		256,
	}
)

// StringToSha1String converts the given string into a SHA1 string
func StringToSha1String(s string) string {
	hash := sha1.New()
	hash.Write([]byte(s))
	return fmt.Sprintf("%x", hash.Sum(nil))
}

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

// Converts an array of notification state strings into a bit mask
func NotificationStatesToBitMask(states []string) int {
	mask := 0
	for _, s := range states {
		mask += NotificationStates[s]
	}
	return mask
}

// Converts an array of notification type strings into a bit mask
func NotificationTypesToBitMask(types []string) int {
	mask := 0
	for _, t := range types {
		mask += NotificationTypes[t]
	}
	return mask
}

func IcingaStateTypeToString(stateType float32) string {
	if stateType == 0 {
		return "hard"
	} else {
		return "soft"
	}
}