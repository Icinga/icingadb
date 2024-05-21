package icingadb

import (
	"context"
	"encoding/binary"
	"github.com/icinga/icingadb/pkg/common"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/database"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/icinga/icingadb/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestDelta(t *testing.T) {
	type TestData struct {
		Name                   string // name for the sub-test
		Actual, Desired        uint64 // checksum to send to actual/desired
		Create, Update, Delete uint64 // checksum that must be in the corresponding map (if != 0)
	}

	tests := []TestData{{
		Name: "Empty",
	}, {
		Name:    "Create",
		Desired: 0x1111111111111111,
		Create:  0x1111111111111111,
	}, {
		Name:    "Update",
		Actual:  0x1111111111111111,
		Desired: 0x2222222222222222,
		Update:  0x2222222222222222,
	}, {
		Name:   "Delete",
		Actual: 0x1111111111111111,
		Delete: 0x1111111111111111,
	}, {
		Name:    "Keep",
		Actual:  0x1111111111111111,
		Desired: 0x1111111111111111,
	}}

	makeEndpoint := func(id, checksum uint64) *v1.Endpoint {
		e := new(v1.Endpoint)
		e.Id = testDeltaMakeIdOrChecksum(id)
		e.PropertiesChecksum = testDeltaMakeIdOrChecksum(checksum)
		return e
	}

	// Send the entities to the actual and desired channels in different ordering to catch bugs in the implementation
	// that only show depending on the order in which actual and desired values are processed for an ID.
	type SendOrder struct {
		Name string
		Send func(id uint64, test TestData, chActual, chDesired chan<- database.Entity)
	}
	sendOrders := []SendOrder{{
		Name: "ActualFirst",
		Send: func(id uint64, test TestData, chActual, chDesired chan<- database.Entity) {
			if test.Actual != 0 {
				chActual <- makeEndpoint(id, test.Actual)
			}
			if test.Desired != 0 {
				chDesired <- makeEndpoint(id, test.Desired)
			}
		},
	}, {
		Name: "DesiredFirst",
		Send: func(id uint64, test TestData, chActual, chDesired chan<- database.Entity) {
			if test.Desired != 0 {
				chDesired <- makeEndpoint(id, test.Desired)
			}
			if test.Actual != 0 {
				chActual <- makeEndpoint(id, test.Actual)
			}
		},
	}}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			for _, sendOrder := range sendOrders {
				t.Run(sendOrder.Name, func(t *testing.T) {
					id := uint64(0x42)
					chActual := make(chan database.Entity)
					chDesired := make(chan database.Entity)
					subject := common.NewSyncSubject(v1.NewEndpoint)
					logger := logging.NewLogger(zaptest.NewLogger(t).Sugar(), time.Second)

					go func() {
						sendOrder.Send(id, test, chActual, chDesired)
						close(chActual)
						close(chDesired)
					}()

					delta := NewDelta(context.Background(), chActual, chDesired, subject, logger)
					err := delta.Wait()
					require.NoError(t, err, "delta should finish without error")

					_, ok := <-chActual
					require.False(t, ok, "chActual should have been closed")
					_, ok = <-chDesired
					require.False(t, ok, "chDesired should have been closed")

					testDeltaVerifyResult(t, "Create", testDeltaMakeExpectedMap(id, test.Create), delta.Create)
					testDeltaVerifyResult(t, "Update", testDeltaMakeExpectedMap(id, test.Update), delta.Update)
					testDeltaVerifyResult(t, "Delete", testDeltaMakeExpectedMap(id, test.Delete), delta.Delete)
				})
			}
		})
	}

	t.Run("Combined", func(t *testing.T) {
		chActual := make(chan database.Entity)
		chDesired := make(chan database.Entity)
		subject := common.NewSyncSubject(v1.NewEndpoint)
		logger := logging.NewLogger(zaptest.NewLogger(t).Sugar(), time.Second)

		expectedCreate := make(map[uint64]uint64)
		expectedUpdate := make(map[uint64]uint64)
		expectedDelete := make(map[uint64]uint64)

		nextId := uint64(1)
		var wg sync.WaitGroup
		for _, test := range tests {
			test := test
			for _, sendOrder := range sendOrders {
				sendOrder := sendOrder
				id := nextId
				nextId++
				// Log ID mapping to allow easier debugging in case of failures.
				t.Logf("ID=%d(%s) Test=%s SendOrder=%s",
					id, testDeltaMakeIdOrChecksum(id).String(), test.Name, sendOrder.Name)
				wg.Add(1)
				go func() {
					defer wg.Done()
					sendOrder.Send(id, test, chActual, chDesired)
				}()

				if test.Create != 0 {
					expectedCreate[id] = test.Create
				}
				if test.Update != 0 {
					expectedUpdate[id] = test.Update
				}
				if test.Delete != 0 {
					expectedDelete[id] = test.Delete
				}
			}
		}
		go func() {
			wg.Wait()
			close(chActual)
			close(chDesired)
		}()

		delta := NewDelta(context.Background(), chActual, chDesired, subject, logger)
		err := delta.Wait()
		require.NoError(t, err, "delta should finish without error")

		_, ok := <-chActual
		require.False(t, ok, "chActual should have been closed")
		_, ok = <-chDesired
		require.False(t, ok, "chDesired should have been closed")

		testDeltaVerifyResult(t, "Create", expectedCreate, delta.Create)
		testDeltaVerifyResult(t, "Update", expectedUpdate, delta.Update)
		testDeltaVerifyResult(t, "Delete", expectedDelete, delta.Delete)
	})
}

func testDeltaMakeIdOrChecksum(i uint64) types.Binary {
	b := make([]byte, 20)
	binary.BigEndian.PutUint64(b, i)
	return b
}

func testDeltaMakeExpectedMap(id uint64, checksum uint64) map[uint64]uint64 {
	if checksum == 0 {
		return nil
	} else {
		return map[uint64]uint64{
			id: checksum,
		}
	}
}

func testDeltaVerifyResult(t *testing.T, name string, expected map[uint64]uint64, got EntitiesById) {
	for id, checksum := range expected {
		idKey := testDeltaMakeIdOrChecksum(id).String()
		if assert.Containsf(t, got, idKey, "%s: should contain %s", name, idKey) {
			expectedChecksum := testDeltaMakeIdOrChecksum(checksum).String()
			gotChecksum := got[idKey].(contracts.Checksumer).Checksum().String()
			assert.Equalf(t, expectedChecksum, gotChecksum, "%s: %s should match checksum", name, idKey)
			delete(got, idKey)
		}
	}

	for id := range got {
		assert.Failf(t, "unexpected element", "%s: should not contain %s", name, id)
	}
}

func BenchmarkDelta(b *testing.B) {
	for n := 1 << 10; n <= 1<<20; n <<= 1 {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			benchmarkDelta(b, n)
		})
	}
}

func benchmarkDelta(b *testing.B, numEntities int) {
	chActual := make([]chan database.Entity, b.N)
	chDesired := make([]chan database.Entity, b.N)
	for i := 0; i < b.N; i++ {
		chActual[i] = make(chan database.Entity, numEntities)
		chDesired[i] = make(chan database.Entity, numEntities)
	}
	makeEndpoint := func(id1, id2, checksum uint64) *v1.Endpoint {
		e := new(v1.Endpoint)
		e.Id = make([]byte, 20)
		binary.BigEndian.PutUint64(e.Id[0:], id1)
		binary.BigEndian.PutUint64(e.Id[8:], id2)
		e.PropertiesChecksum = make([]byte, 20)
		binary.BigEndian.PutUint64(e.PropertiesChecksum, checksum)
		return e
	}
	for i := 0; i < numEntities; i++ {
		// each iteration writes exactly one entity to each channel
		var eActual, eDesired database.Entity
		switch i % 3 {
		case 0: // distinct IDs
			eActual = makeEndpoint(1, uint64(i), uint64(i))
			eDesired = makeEndpoint(2, uint64(i), uint64(i))
		case 1: // same ID, same checksum
			e := makeEndpoint(3, uint64(i), uint64(i))
			eActual = e
			eDesired = e
		case 2: // same ID, different checksum
			eActual = makeEndpoint(4, uint64(i), uint64(i))
			eDesired = makeEndpoint(4, uint64(i), uint64(i+1))
		}
		for _, ch := range chActual {
			ch <- eActual
		}
		for _, ch := range chDesired {
			ch <- eDesired
		}
	}
	for i := 0; i < b.N; i++ {
		close(chActual[i])
		close(chDesired[i])
	}
	subject := common.NewSyncSubject(v1.NewEndpoint)
	// logger := zaptest.NewLogger(b).Sugar()
	logger := logging.NewLogger(zap.New(zapcore.NewTee()).Sugar(), time.Second)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d := NewDelta(context.Background(), chActual[i], chDesired[i], subject, logger)
		err := d.Wait()
		assert.NoError(b, err, "delta should not fail")
	}
}
