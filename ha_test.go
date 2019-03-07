package icingadb_ha

import (
	"git.icinga.com/icingadb/icingadb-connection"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var testID, _ = uuid.FromBytes(make([]byte, 16))
var testEnv = make([]byte, 20)

func TestHA_setResponsibility(t *testing.T) {
	responsibilities := [6]int32{ resp_ReadyForTakeover, resp_TakeoverNoSync, resp_TakeoverSync, resp_Stop, resp_NotReadyForTakeover }
	h := new(HA)

	previous := int32(0)
	for _,r := range responsibilities {
		assert.Equal(t, previous, h.setResponsibility(r), "Should be equal")
		previous = r
	}
}

func TestHA_IsResponsible(t *testing.T) {
	h := new(HA)
	h.setResponsibility(resp_TakeoverSync)
	assert.True(t, h.IsResponsible(), "Should be responsible")
	h.setResponsibility(resp_TakeoverNoSync)
	assert.False(t, h.IsResponsible(), "Should not be responsible")
}

func TestHA_icinga2IsAlive(t *testing.T) {
	h := new(HA)
	h.icinga2MTime = time.Now().Unix() - 5
	assert.True(t, h.icinga2IsAlive(), "Should be alive")
	h.icinga2MTime = h.icinga2MTime - 15
	assert.False(t, h.icinga2IsAlive(), "Should be dead")
}

func TestHA_handleResponsibility(t *testing.T) {
	h := new(HA)
	h.ourEnv = &icingadb_connection.Environment{make([]byte, 20), "test"}
	h.setResponsibility(resp_ReadyForTakeover)
	var cont bool
	var na int

	print(h.icinga2MTime)

	cont, na = h.handleResponsibility()
	assert.True(t, cont)
	assert.Equal(t, action_TryTakeover, na)
	assert.Equal(t, resp_NotReadyForTakeover, h.getResponsibility())

	//AWAKEN
	h.icinga2MTime = time.Now().Unix()
	cont, na = h.handleResponsibility()
	assert.True(t, cont)
	assert.Equal(t, resp_ReadyForTakeover, h.getResponsibility())

	h.setResponsibility(resp_TakeoverSync)
	cont, na = h.handleResponsibility()
	assert.False(t, cont)
	assert.Equal(t, action_DoTakeover, na)
	assert.Equal(t, resp_TakeoverSync, h.getResponsibility())

	//SLEEP
	h.icinga2MTime = 0
	cont, na = h.handleResponsibility()
	assert.True(t, cont)
	assert.Equal(t, na, action_DoTakeover)
}

func Test_cleanUpInstances(t *testing.T) {
	var dbw, err = icingadb_connection.NewDBWrapper(
		"module-dev:icinga0815!@tcp(127.0.0.1:3306)/icingadb?")
	assert.NoError(t, err, "SQL error")

	_, err = dbw.SqlExec(
		"insert into icingadb_instance",
		`INSERT INTO icingadb_instance(id, environment_id, heartbeat, responsible) VALUES (?, ?, ?, ?)`,
		testID[:],
		testEnv,
		time.Now().Unix() - 25,
		"n",
	)

	assert.NoError(t, err, "SQL error")

	rows, err := dbw.SqlFetchAll("", "SELECT 1 FROM icingadb_instance WHERE id = ?", testID[:])

	assert.Equal(t, 1, len(rows))

	err = cleanUpInstances(dbw)

	assert.NoError(t, err, "Clean up failed")

	rows, err = dbw.SqlFetchAll("", "SELECT 1 FROM icingadb_instance WHERE id = ?", testID[:])

	assert.NoError(t, err, "SQL error")

	assert.Equal(t, 1, len(rows))

	time.Sleep(time.Second * 5)

	err = cleanUpInstances(dbw)

	assert.NoError(t, err, "Clean up failed")

	rows, err = dbw.SqlFetchAll("", "SELECT 1 FROM icingadb_instance WHERE id = ?", testID[:])

	assert.NoError(t, err, "SQL error")

	assert.Equal(t, 0, len(rows))
}