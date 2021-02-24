package history

import (
	"crypto/rand"
	"github.com/Icinga/icingadb/config/testbackends"
	"github.com/Icinga/icingadb/connection"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strconv"
	"sync"
	"testing"
)

// TODO: this tests only the SQL stored procedure, not the code inserting into sla_history

var mysqlTestObserver = connection.DbIoSeconds.WithLabelValues("mysql", "test")

type Checkable struct {
	EnvironmentId []byte
	EndpointId    []byte
	ObjectType    string
	ObjectId      []byte
}

type SlaEvent interface {
	WriteSlaHistoryToDatabase(db *connection.DBWrapper, c *Checkable) error
}

type State struct {
	Time  uint64
	State int
}

var _ SlaEvent = (*State)(nil)

func (s *State) WriteSlaHistoryToDatabase(db *connection.DBWrapper, c *Checkable) error {
	id, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	_, err = db.SqlExec(mysqlTestObserver, "INSERT INTO sla_history_state (id, environment_id, endpoint_id, "+
		"object_type, object_id, event_time, hard_state) VALUES (?,?,?,?,?,?,?)",
		id[:],           // id
		c.EnvironmentId, // environment_id
		c.EndpointId,    // endpoint_id
		c.ObjectType,    // object_type
		c.ObjectId,      // object_id
		s.Time,          // event_time
		s.State,         // hard_state
	)
	return err
}

type Downtime struct {
	Start uint64
	End   uint64
}

var _ SlaEvent = (*Downtime)(nil)

func (d *Downtime) WriteSlaHistoryToDatabase(db *connection.DBWrapper, c *Checkable) error {
	downtimeId := make([]byte, 20)
	_, err := rand.Read(downtimeId)
	if err != nil {
		return err
	}

	_, err = db.SqlExec(mysqlTestObserver, "INSERT INTO sla_history_downtime (environment_id,"+
		"endpoint_id, object_type, object_id, downtime_id, downtime_start, downtime_end) "+
		"VALUES (?,?,?,?,?,?,?)",
		c.EnvironmentId, // environment_id
		c.EndpointId,    // endpoint_id
		c.ObjectType,    // object_type
		c.ObjectId,      // object_id
		downtimeId,      // downtime_id
		d.Start,         // downtime_start
		d.End,           // downtime_end
	)

	return err
}

func sla(conn *connection.DBWrapper, objectType string, objectId []byte, start uint64, end uint64) (float64, error) {
	type row struct {
		Sla string
	}
	rows, err := conn.SqlFetchAll(mysqlTestObserver, row{},
		// Go does not like DECIMAL and FLOAT/DOUBLE behaves strange across MySQL/MariaDB versions...
		"SELECT CONCAT(reports_get_sla_ok_percent(?, ?, ?, ?))",
		objectType, objectId, start, end)
	if err != nil {
		return -1, err
	}
	s, err := strconv.ParseFloat(rows.([]row)[0].Sla, 64)
	if err != nil {
		return -1, err
	}
	return s, nil
}

func testSla(t *testing.T, events []SlaEvent, start uint64, end uint64, expected float64, msg string) {
	t.Run("Host", func(t *testing.T) {
		testSlaWithObjectType(t, events, false, start, end, expected, msg)
	})
	t.Run("Service", func(t *testing.T) {
		testSlaWithObjectType(t, events, true, start, end, expected, msg)
	})
}

var truncateOnce sync.Once

func testSlaWithObjectType(t *testing.T,
	events []SlaEvent, service bool, start uint64, end uint64, expected float64, msg string,
) {
	mysqlConn, err := connection.NewDBWrapper(testbackends.MysqlTestDsn, 50)
	require.NoError(t, err, "This test needs a working MySQL connection!")

	truncateOnce.Do(func() {
		for _, table := range []string{"sla_history_downtime", "sla_history_state"} {
			_, err = mysqlConn.SqlExec(mysqlTestObserver, "TRUNCATE "+table)
			require.NoErrorf(t, err, "Truncating %s failed", table)
		}
	})

	var objectType string
	if service {
		objectType = "service"
	} else {
		objectType = "host"
	}

	checkable := &Checkable{
		EnvironmentId: make([]byte, 20),
		EndpointId:    make([]byte, 20),
		ObjectType:    objectType,
		ObjectId:      make([]byte, 20),
	}

	_, err = rand.Read(checkable.EndpointId)
	require.NoError(t, err, "generating environment_id failed")
	_, err = rand.Read(checkable.EndpointId)
	require.NoError(t, err, "generating endpoint_id failed")
	_, err = rand.Read(checkable.ObjectId)
	require.NoError(t, err, "generating object_id failed")

	for _, event := range events {
		err := event.WriteSlaHistoryToDatabase(mysqlConn, checkable)
		require.NoErrorf(t, err, "Inserting SLA history for %#v failed", event)
	}

	r, err := sla(mysqlConn, objectType, checkable.ObjectId, start, end)
	require.NoError(t, err, "SLA query should not fail")
	assert.Equal(t, expected, r, msg)
}

func TestSla(t *testing.T) {
	// SLA should be 90%:
	// 1000..1100: OK, no downtime
	// 1100..1200: OK, in downtime
	// 1200..1300: CRITICAL, in downtime
	// 1300..1400: CRITICAL, no downtime ***
	// 1400..1500: CRITICAL, in downtime
	// 1500..1600: OK, in downtime
	// 1600..2000: OK, no downtime
	events := []SlaEvent{
		&Downtime{Start: 1100, End: 1300},
		&Downtime{Start: 1400, End: 1600},
		&State{Time: 1200, State: 2},
		&State{Time: 1500, State: 0},
	}
	testSla(t, events, 1000, 2000, 90.0, "SLA should be 90%")
}

func TestSlaEmptyHistory(t *testing.T) {
	// Empty history implies no previous problem state, therefore SLA should be 100%
	testSla(t, nil, 1000, 2000, 100.0, "SLA should be 100%")
}

func TestSlaCriticalBeforeInterval(t *testing.T) {
	events := []SlaEvent{
		&State{Time: 0, State: 2},
	}
	testSla(t, events, 1000, 2000, 0.0, "SLA should be 0%")
}

func TestSlaCriticalBeforeIntervalWithDowntime(t *testing.T) {
	events := []SlaEvent{
		&State{Time: 800, State: 2},
		&Downtime{Start: 600, End: 1800},
	}
	testSla(t, events, 1000, 2000, 80.0, "SLA should be 80%")
}

func TestSlaCriticalBeforeIntervalWithOverlappingDowntimes(t *testing.T) {
	events := []SlaEvent{
		&State{Time: 800, State: 2},
		&Downtime{Start: 600, End: 1000},
		&Downtime{Start: 800, End: 1200},
		&Downtime{Start: 1000, End: 1400},
		&Downtime{Start: 1600, End: 2000},
		&Downtime{Start: 1800, End: 2200},
	}
	testSla(t, events, 1000, 2000, 80.0, "SLA should be 80%")
}
