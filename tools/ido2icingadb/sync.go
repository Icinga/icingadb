package main

import (
	"bytes"
	"encoding/binary"
	"github.com/google/uuid"
)

// historyTable represents Icinga DB history tables.
type historyTable byte

const (
	stateHistory historyTable = 's'
)

// uuidTemplate is for mkDeterministicUuid.
var uuidTemplate = func() uuid.UUID {
	buf := &bytes.Buffer{}
	buf.Write(uuid.Nil[:])

	uid, errNR := uuid.NewRandomFromReader(buf)
	if errNR != nil {
		panic(errNR)
	}

	copy(uid[:], "IDO h")

	return uid
}()

// mkDeterministicUuid returns a formally random UUID (v4) as follows: 11111122-3300-4455-4455-555555555555
//
// 0: zeroed
// 1: "IDO" (where the data identified by the new UUID is from)
// 2: the history table the new UUID is for, e.g. "s" for state_history
// 3: "h" (for "history")
// 4: the new UUID's formal version (unused bits zeroed)
// 5: the ID of the row the new UUID is for in the IDO (big endian)
func mkDeterministicUuid(table historyTable, rowId uint64) uuid.UUID {
	uid := uuidTemplate
	uid[3] = byte(table)

	buf := &bytes.Buffer{}
	if errWB := binary.Write(buf, binary.BigEndian, rowId); errWB != nil {
		panic(errWB)
	}

	bEId := buf.Bytes()
	uid[7] = bEId[0]
	copy(uid[9:], bEId[1:])

	return uid
}
