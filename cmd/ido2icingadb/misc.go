package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"github.com/google/uuid"
	"github.com/icinga/icingadb/pkg/icingadb/objectpacker"
	"github.com/jmoiron/sqlx"
	"github.com/vbauerster/mpb/v6"
	"golang.org/x/sync/errgroup"
)

type ProgressRow struct {
	Id   uint64
	Name string
}

type convertedId struct {
	ido uint64
	idb []byte
}

// mkDeterministicUuid returns a formally random UUID (v4) as follows: 11111122-3300-4455-4455-555555555555
//
// 0: zeroed
// 1: "IDO" (where the data identified by the new UUID is from)
// 2: the history table the new UUID is for, e.g. "s" for state_history
// 3: "h" (for "history")
// 4: the new UUID's formal version (unused bits zeroed)
// 5: the ID of the row the new UUID is for in the IDO (big endian)
func mkDeterministicUuid(table byte, rowId uint64) []byte {
	uid := uuidTemplate
	uid[3] = table

	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.BigEndian, rowId); err != nil {
		panic(err)
	}

	bEId := buf.Bytes()
	uid[7] = bEId[0]
	copy(uid[9:], bEId[1:])

	return uid[:]
}

// uuidTemplate is for mkDeterministicUuid.
var uuidTemplate = func() uuid.UUID {
	buf := &bytes.Buffer{}
	buf.Write(uuid.Nil[:])

	uid, err := uuid.NewRandomFromReader(buf)
	if err != nil {
		panic(err)
	}

	copy(uid[:], "IDO h")
	return uid
}()

// hashAny combines PackAny and SHA1 hashing.
func hashAny(in interface{}) []byte {
	hash := sha1.New()
	if err := objectpacker.PackAny(in, hash); err != nil {
		panic(err)
	}

	return hash.Sum(nil)
}

// calcObjectId calculates the ID of the config object named name1 for Icinga DB.
func calcObjectId(env, name1 string) []byte {
	return hashAny([2]string{env, name1})
}

type historyType struct {
	name        string
	idoTable    string
	idoIdColumn string
	idoColumns  []string
	idbTable    string
	idbIdColumn string
	convertId   func(row ProgressRow, env string) []byte
	snapshot    *sqlx.Tx
	total       int64
	bar         *mpb.Bar
	lastId      uint64
}

type historyTypes [6]historyType

func (ht *historyTypes) forEach(f func(*historyType)) {
	eg, _ := errgroup.WithContext(context.Background())
	for i := range *ht {
		i := i
		eg.Go(func() error {
			f(&(*ht)[i])
			return nil
		})
	}

	_ = eg.Wait()
}

var types = historyTypes{
	{
		"acknowledgement",
		"icinga_acknowledgements",
		"acknowledgement_id",
		nil,
		"history",
		"id",
		func(row ProgressRow, _ string) []byte { return mkDeterministicUuid('a', row.Id) },
		nil, 0, nil, 0,
	},
	{
		"comment",
		"icinga_commenthistory",
		"commenthistory_id",
		[]string{"name"},
		"comment_history",
		"comment_id",
		func(row ProgressRow, env string) []byte { return calcObjectId(env, row.Name) },
		nil, 0, nil, 0,
	},
	{
		"downtime",
		"icinga_downtimehistory",
		"downtimehistory_id",
		[]string{"name"},
		"downtime_history",
		"downtime_id",
		func(row ProgressRow, env string) []byte { return calcObjectId(env, row.Name) },
		nil, 0, nil, 0,
	},
	{
		"flapping",
		"icinga_flappinghistory",
		"flappinghistory_id",
		nil,
		"history",
		"id",
		func(row ProgressRow, _ string) []byte { return mkDeterministicUuid('f', row.Id) },
		nil, 0, nil, 0,
	},
	{
		"notification",
		"icinga_notifications",
		"notification_id",
		nil,
		"notification_history",
		"id",
		func(row ProgressRow, _ string) []byte { return mkDeterministicUuid('n', row.Id) },
		nil, 0, nil, 0,
	},
	{
		"state",
		"icinga_statehistory",
		"statehistory_id",
		nil,
		"state_history",
		"id",
		func(row ProgressRow, _ string) []byte { return mkDeterministicUuid('s', row.Id) },
		nil, 0, nil, 0,
	},
}
