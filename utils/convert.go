// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package utils

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math"
	"time"
)

var (
	Bool = map[bool]string{
		true:  "y",
		false: "n",
	}
	IsAcknowledged = map[float32]string{
		0: "n",
		1: "y",
		2: "sticky",
	}
	NotificationStatesToInt = map[string]int{
		"OK":       1,
		"Warning":  2,
		"Critical": 4,
		"Unknown":  8,
		"Up":       16,
		"Down":     32,
	}
	NotificationTypesToInt = map[string]int{
		"DowntimeStart":   1,
		"DowntimeEnd":     2,
		"DowntimeRemoved": 4,
		"Custom":          8,
		"Acknowledgement": 16,
		"Problem":         32,
		"Recovery":        64,
		"FlappingStart":   128,
		"FlappingEnd":     256,
	}
	NotificationTypesToDbEnumString = map[string]string{
		"1":   "downtime_start",
		"2":   "downtime_end",
		"4":   "downtime_removed",
		"8":   "custom",
		"16":  "acknowledgement",
		"32":  "problem",
		"64":  "recovery",
		"128": "flapping_start",
		"256": "flapping_end",
	}
	CommentEntryTypes = map[string]string{
		"1": "comment",
		"4": "ack",
	}
)

// Checksum converts the given string into a SHA1 checksum string.
func Checksum(s string) string {
	hash := sha1.New()
	hash.Write([]byte(s))
	return fmt.Sprintf("%x", hash.Sum(nil))
}

// EncodeChecksum converts a hex string to a byte array.
func EncodeChecksum(s string) []byte {
	c, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}

	return c
}

// DecodeHexIfNotNil converts a hex string to a byte array.
func DecodeHexIfNotNil(hexStr interface{}) interface{} {
	if hexStr == nil {
		return nil
	}

	return EncodeChecksum(hexStr.(string))
}

// DecodeChecksum coverts a byte array into a hex string.
func DecodeChecksum(c []byte) string {
	return hex.EncodeToString(c)
}

// NotificationStatesToBitMask converts an array of notification state strings into a bit mask.
func NotificationStatesToBitMask(states []string) int {
	if states == nil {
		return 63
	}

	mask := 0
	for _, s := range states {
		mask += NotificationStatesToInt[s]
	}
	return mask
}

// NotificationTypesToBitMask converts an array of notification type strings into a bit mask.
func NotificationTypesToBitMask(types []string) int {
	if types == nil {
		return 511
	}

	mask := 0
	for _, t := range types {
		mask += NotificationTypesToInt[t]
	}
	return mask
}

func IcingaStateTypeToString(stateType float32) string {
	if stateType == 0 {
		return "soft"
	} else {
		return "hard"
	}
}

// JSONBooleanToDBBoolean converts a boolean we got from Redis into a DB boolean.
func JSONBooleanToDBBoolean(value interface{}) string {
	if value == "true" {
		return "y"
	} else { //Should catch empty strings and nil
		return "n"
	}
}

func RedisIntToDBBoolean(value interface{}) string {
	if value == "1" {
		return "y"
	} else { //Should catch empty strings and nil
		return "n"
	}
}

func MillisecsToTime(millis float64) time.Time {
	secs := millis / 1000
	wholeSecs := math.Floor(secs)

	return time.Unix(int64(wholeSecs), int64((secs-wholeSecs)*(float64(time.Second)/float64(time.Nanosecond))))
}

// TimeToMillisecs returns t as ms since *nix epoch.
func TimeToMillisecs(t time.Time) int64 {
	sec := t.Unix()
	return sec*1000 + int64(t.Sub(time.Unix(sec, 0))/time.Millisecond)
}

func TimeToFloat(t time.Time) float64 {
	secs := t.Unix()
	return float64(secs) + float64(t.Sub(time.Unix(secs, 0)))/float64(time.Second)
}

// DefaultIfNil returns a defaultValue, if the given value is nil
func DefaultIfNil(value, defaultValue interface{}) interface{} {
	if value != nil {
		return value
	} else {
		return defaultValue
	}
}