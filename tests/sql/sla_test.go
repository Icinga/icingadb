package sql_test

import (
	"crypto/rand"
	"database/sql/driver"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSla(t *testing.T) {
	rdb := getDatabase(t)
	db, err := sqlx.Open(rdb.Driver(), rdb.DSN())
	require.NoError(t, err, "connect to database")

	type TestData struct {
		Name     string
		Events   []SlaHistoryEvent
		Start    uint64
		End      uint64
		Expected float64
	}

	tests := []TestData{{
		Name: "EmptyHistory",
		// Empty history implies no previous problem state, therefore SLA should be 100%
		Events:   nil,
		Start:    1000,
		End:      2000,
		Expected: 100.0,
	}, {
		Name: "MultipleStateChanges",
		// Some flapping, test that all changes are considered.
		Events: []SlaHistoryEvent{
			&State{Time: 1000, State: 2, PreviousState: 99}, // -10%
			&State{Time: 1100, State: 0, PreviousState: 2},
			&State{Time: 1300, State: 2, PreviousState: 0}, // -10%
			&State{Time: 1400, State: 0, PreviousState: 2},
			&State{Time: 1600, State: 2, PreviousState: 0}, // -10%
			&State{Time: 1700, State: 0, PreviousState: 2},
			&State{Time: 1900, State: 2, PreviousState: 0}, // -10%
		},
		Start:    1000,
		End:      2000,
		Expected: 60.0,
	}, {
		Name: "OverlappingDowntimesAndProblems",
		// SLA should be 90%:
		// 1000..1100: OK, no downtime
		// 1100..1200: OK, in downtime
		// 1200..1300: CRITICAL, in downtime
		// 1300..1400: CRITICAL, no downtime (only period counting for SLA, -10%)
		// 1400..1500: CRITICAL, in downtime
		// 1500..1600: OK, in downtime
		// 1600..2000: OK, no downtime
		Events: []SlaHistoryEvent{
			&Downtime{Start: 1100, End: 1300},
			&Downtime{Start: 1400, End: 1600},
			&State{Time: 1200, State: 2, PreviousState: 0},
			&State{Time: 1500, State: 0, PreviousState: 2},
		},
		Start:    1000,
		End:      2000,
		Expected: 90.0,
	}, {
		Name: "CriticalBeforeInterval",
		// If there is no event within the SLA interval, the last state from before the interval should be used.
		Events: []SlaHistoryEvent{
			&State{Time: 0, State: 2, PreviousState: 99},
		},
		Start:    1000,
		End:      2000,
		Expected: 0.0,
	}, {
		Name: "CriticalBeforeIntervalWithDowntime",
		// State change and downtime start from before the SLA interval should be considered if still relevant.
		Events: []SlaHistoryEvent{
			&State{Time: 800, State: 2, PreviousState: 99},
			&Downtime{Start: 600, End: 1800},
		},
		Start:    1000,
		End:      2000,
		Expected: 80.0,
	}, {
		Name: "CriticalBeforeIntervalWithOverlappingDowntimes",
		// Test that overlapping downtimes are properly accounted for.
		Events: []SlaHistoryEvent{
			&State{Time: 800, State: 2, PreviousState: 99},
			&Downtime{Start: 600, End: 1000},
			&Downtime{Start: 800, End: 1200},
			&Downtime{Start: 1000, End: 1400},
			// Everything except 1400-1600 is covered by downtimes, -20%
			&Downtime{Start: 1600, End: 2000},
			&Downtime{Start: 1800, End: 2200},
		},
		Start:    1000,
		End:      2000,
		Expected: 80.0,
	}, {
		Name: "FallbackToPreviousState",
		// If there is no state event from before the SLA interval, the previous hard state from the first event
		// after the beginning of the SLA interval should be used as the initial state.
		Events: []SlaHistoryEvent{
			&State{Time: 1200, State: 0, PreviousState: 2},
		},
		Start:    1000,
		End:      2000,
		Expected: 80.0,
	}, {
		Name: "FallbackToCurrentState",
		// If there are no state history events, the current state of the checkable should be used.
		Events: []SlaHistoryEvent{
			&CurrentState{State: 2},
		},
		Start:    1000,
		End:      2000,
		Expected: 0.0,
	}, {
		Name: "PreferInitialStateFromBeforeOverLaterState",
		// The previous_hard_state should only be used as a fallback when there is no event from before the
		// SLA interval. Therefore, the latter should be preferred if there is conflicting information.
		Events: []SlaHistoryEvent{
			&State{Time: 800, State: 2, PreviousState: 99},
			&State{Time: 1200, State: 0, PreviousState: 0},
		},
		Start:    1000,
		End:      2000,
		Expected: 80.0,
	}, {
		Name: "PreferInitialStateFromBeforeOverCurrentState",
		// The current state should only be used as a fallback when there is no state history event.
		// Therefore, the latter should be preferred if there is conflicting information.
		Events: []SlaHistoryEvent{
			&State{Time: 800, State: 2, PreviousState: 99},
			&CurrentState{State: 0},
		},
		Start:    1000,
		End:      2000,
		Expected: 0.0,
	}, {
		Name: "PreferLaterStateOverCurrentState",
		// The current state should only be used as a fallback when there is no state history event.
		// Therefore, the latter should be preferred if there is conflicting information.
		Events: []SlaHistoryEvent{
			&State{Time: 1200, State: 0, PreviousState: 2},
			&CurrentState{State: 2},
		},
		Start:    1000,
		End:      2000,
		Expected: 80.0,
	}, {
		Name: "InitialUnknownReducesTotalTime",
		Events: []SlaHistoryEvent{
			&State{Time: 1500, State: 2, PreviousState: 99},
			&State{Time: 1700, State: 0, PreviousState: 2},
			&CurrentState{State: 0},
		},
		Start:    1000,
		End:      2000,
		Expected: 60,
	}, {
		Name: "IntermediateUnknownReducesTotalTime",
		Events: []SlaHistoryEvent{
			&State{Time: 1000, State: 0, PreviousState: 2},
			&State{Time: 1100, State: 2, PreviousState: 0},
			&State{Time: 1600, State: 0, PreviousState: 99},
			&State{Time: 1800, State: 2, PreviousState: 0},
			&CurrentState{State: 0},
		},
		Start:    1000,
		End:      2000,
		Expected: 60,
	}}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			testSla(t, db, test.Events, test.Start, test.End, test.Expected, "unexpected SLA value")
		})
	}

	t.Run("Invalid", func(t *testing.T) {
		m := SlaHistoryMeta{
			EnvironmentId: make([]byte, 20),
			EndpointId:    make([]byte, 20),
			ObjectType:    "host",
			HostId:        make([]byte, 20),
		}

		checkErr := func(t *testing.T, err error) {
			require.Error(t, err, "SLA function should return an error")

			switch d := db.DriverName(); d {
			case "mysql":
				var mysqlErr *mysql.MySQLError
				require.ErrorAs(t, err, &mysqlErr, "SLA function should return a MySQL error")
				// https://dev.mysql.com/doc/mysql-errors/8.0/en/server-error-reference.html#error_er_signal_exception
				assert.Equal(t, uint16(1644), mysqlErr.Number, "MySQL error should be ER_SIGNAL_EXCEPTION")
				assert.Equal(t, "end time must be greater than start time", mysqlErr.Message,
					"MySQL error should contain custom message")

			case "postgres":
				var pqErr *pq.Error
				require.ErrorAs(t, err, &pqErr, "SLA function should return a PostgreSQL error")
				assert.Equal(t, pq.ErrorCode("P0001"), pqErr.Code, "MySQL error should be ER_SIGNAL_EXCEPTION")
				assert.Equal(t, "end time must be greater than start time", pqErr.Message,
					"PostgreSQL error should contain custom message")

			default:
				panic(fmt.Sprintf("unknown database driver %q", d))
			}
		}

		t.Run("ZeroDuration", func(t *testing.T) {
			_, err := execSqlSlaFunc(db, &m, 1000, 1000)
			checkErr(t, err)
		})

		t.Run("NegativeDuration", func(t *testing.T) {
			_, err := execSqlSlaFunc(db, &m, 2000, 1000)
			checkErr(t, err)
		})
	})
}

func execSqlSlaFunc(db *sqlx.DB, m *SlaHistoryMeta, start uint64, end uint64) (float64, error) {
	var result float64
	err := db.Get(&result, db.Rebind("SELECT get_sla_ok_percent(?, ?, ?, ?)"),
		m.HostId, m.ServiceId, start, end)
	return result, err
}

func testSla(t *testing.T, db *sqlx.DB, events []SlaHistoryEvent, start uint64, end uint64, expected float64, msg string) {
	t.Run("Host", func(t *testing.T) {
		testSlaWithObjectType(t, db, events, false, start, end, expected, msg)
	})
	t.Run("Service", func(t *testing.T) {
		testSlaWithObjectType(t, db, events, true, start, end, expected, msg)
	})
}

func testSlaWithObjectType(t *testing.T, db *sqlx.DB,
	events []SlaHistoryEvent, service bool, start uint64, end uint64, expected float64, msg string,
) {
	makeId := func() []byte {
		id := make([]byte, 20)
		_, err := rand.Read(id)
		require.NoError(t, err, "generating random id failed")
		return id
	}

	meta := SlaHistoryMeta{
		EnvironmentId: makeId(),
		EndpointId:    makeId(),
		HostId:        makeId(),
	}
	if service {
		meta.ObjectType = "service"
		meta.ServiceId = makeId()
	} else {
		meta.ObjectType = "host"
	}

	for _, event := range events {
		err := event.WriteSlaEventToDatabase(db, &meta)
		require.NoErrorf(t, err, "Inserting SLA history for %#v failed", event)
	}

	r, err := execSqlSlaFunc(db, &meta, start, end)
	require.NoError(t, err, "SLA query should not fail")
	assert.Equal(t, expected, r, msg)
}

type SlaHistoryMeta struct {
	EnvironmentId NullableBytes `db:"environment_id"`
	EndpointId    NullableBytes `db:"endpoint_id"`
	ObjectType    string        `db:"object_type"`
	HostId        NullableBytes `db:"host_id"`
	ServiceId     NullableBytes `db:"service_id"`
}

type SlaHistoryEvent interface {
	WriteSlaEventToDatabase(db *sqlx.DB, m *SlaHistoryMeta) error
}

type State struct {
	Time          uint64
	State         uint8
	PreviousState uint8
}

var _ SlaHistoryEvent = (*State)(nil)

func (s *State) WriteSlaEventToDatabase(db *sqlx.DB, m *SlaHistoryMeta) error {
	type values struct {
		*SlaHistoryMeta
		Id                []byte `db:"id"`
		EventTime         uint64 `db:"event_time"`
		HardState         uint8  `db:"hard_state"`
		PreviousHardState uint8  `db:"previous_hard_state"`
	}

	id := make([]byte, 20)
	_, err := rand.Read(id)
	if err != nil {
		return err
	}

	_, err = db.NamedExec("INSERT INTO sla_history_state"+
		" (id, environment_id, endpoint_id, object_type, host_id, service_id, event_time, hard_state, previous_hard_state)"+
		" VALUES (:id, :environment_id, :endpoint_id, :object_type, :host_id, :service_id, :event_time, :hard_state, :previous_hard_state)",
		&values{
			SlaHistoryMeta:    m,
			Id:                id[:],
			EventTime:         s.Time,
			HardState:         s.State,
			PreviousHardState: s.PreviousState,
		})
	return err
}

type CurrentState struct {
	State uint8
}

func (c *CurrentState) WriteSlaEventToDatabase(db *sqlx.DB, m *SlaHistoryMeta) error {
	type values struct {
		*SlaHistoryMeta
		State              uint8         `db:"state"`
		PropertiesChecksum NullableBytes `db:"properties_checksum"`
	}

	v := values{
		SlaHistoryMeta:     m,
		State:              c.State,
		PropertiesChecksum: make([]byte, 20),
	}

	if len(m.ServiceId) == 0 {
		_, err := db.NamedExec("INSERT INTO host_state"+
			" (id, host_id, environment_id, properties_checksum, soft_state, previous_soft_state,"+
			" hard_state, previous_hard_state, attempt, severity, last_state_change, next_check, next_update)"+
			" VALUES (:host_id, :host_id, :environment_id, :properties_checksum, :state, :state, :state, :state, 0, 0, 0, 0, 0)",
			&v)
		return err
	} else {
		_, err := db.NamedExec("INSERT INTO service_state"+
			" (id, host_id, service_id, environment_id, properties_checksum, soft_state, previous_soft_state,"+
			" hard_state, previous_hard_state, attempt, severity, last_state_change, next_check, next_update)"+
			" VALUES (:service_id, :host_id, :service_id, :environment_id, :properties_checksum, :state, :state, :state, :state, 0, 0, 0, 0, 0)",
			&v)
		return err
	}
}

var _ SlaHistoryEvent = (*CurrentState)(nil)

type Downtime struct {
	Start uint64
	End   uint64
}

var _ SlaHistoryEvent = (*Downtime)(nil)

type slaHistoryDowntime struct {
	*SlaHistoryMeta
	DowntimeId    []byte `db:"downtime_id"`
	DowntimeStart uint64 `db:"downtime_start"`
	DowntimeEnd   uint64 `db:"downtime_end"`
}

func (d *Downtime) WriteSlaEventToDatabase(db *sqlx.DB, m *SlaHistoryMeta) error {
	downtimeId := make([]byte, 20)
	_, err := rand.Read(downtimeId)
	if err != nil {
		return err
	}

	_, err = db.NamedExec("INSERT INTO sla_history_downtime"+
		" (environment_id, endpoint_id, object_type, host_id, service_id, downtime_id, downtime_start, downtime_end)"+
		" VALUES (:environment_id, :endpoint_id, :object_type, :host_id,"+
		"         :service_id, :downtime_id, :downtime_start, :downtime_end)",
		&slaHistoryDowntime{
			SlaHistoryMeta: m,
			DowntimeId:     downtimeId[:],
			DowntimeStart:  d.Start,
			DowntimeEnd:    d.End,
		})
	return err
}

// NullableBytes allows writing to binary columns in a database with support for NULL.
type NullableBytes []byte

// Value implements the database/sql/driver.Valuer interface.
func (b NullableBytes) Value() (driver.Value, error) {
	if b != nil {
		return []byte(b), nil
	}

	// any(nil) is treated as NULL in contrast to []byte(nil) which is a non-NULL byte sequence of length 0.
	return nil, nil
}
