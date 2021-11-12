package icingadb_test

import (
	"github.com/icinga/icinga-testing/utils"
	"github.com/icinga/icinga-testing/utils/eventually"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
	"time"
)

// Regression test for https://github.com/Icinga/icingadb/pull/394
//
// Upsert and delete runtime objects for the same object must be executed in-order. Otherwise, an old update might
// replace the object in the database with an older version or delete an object even if it still exists.
func TestRegression394(t *testing.T) {
	numObjects := 200

	logger := it.Logger(t)

	r := it.RedisServerT(t)
	m := it.MysqlDatabaseT(t)
	i := it.Icinga2NodeT(t, "master")
	i.EnableIcingaDb(r)
	err := i.Reload()
	require.NoError(t, err, "icinga2 reload")

	// Wait for Icinga 2 to signal a successful dump before starting
	// Icinga DB to ensure that we actually test the initial sync.
	logger.Debug("waiting for icinga2 dump done signal")
	waitForDumpDoneSignal(t, r, 20*time.Second, 100*time.Millisecond)

	// Only after that, start Icinga DB.
	logger.Debug("starting icingadb")
	it.IcingaDbInstanceT(t, r, m)

	client := i.ApiClient()

	db, err := sqlx.Open("mysql", m.DSN())
	require.NoError(t, err, "connecting to mysql shouldn't fail")
	t.Cleanup(func() { _ = db.Close() })

	waitForPendingRuntimeUpdates := func(t *testing.T) {
		// To verify that all host runtime updates up to now have been processed, create a marker host, wait for it to
		// appear in the database, delete it, and wait for it to disappear again.
		markerName := utils.UniqueName(t, "marker")

		client.CreateHost(t, markerName, nil)
		eventually.Require(t, func(t require.TestingT) {
			var count int
			err = db.Get(&count, "SELECT COUNT(*) FROM host WHERE name = ?", markerName)
			require.NoError(t, err, "select host count from database")
			assert.Equalf(t, 1, count, "marker host %q should appear in database", markerName)
		}, 10*time.Second, 100*time.Millisecond)

		client.DeleteHost(t, markerName, true)
		eventually.Require(t, func(t require.TestingT) {
			var count int
			err = db.Get(&count, "SELECT COUNT(*) FROM host WHERE name = ?", markerName)
			require.NoError(t, err, "select host count from database")
			assert.Zerof(t, count, "marker host %q should disappear from database", markerName)
		}, 10*time.Second, 100*time.Millisecond)
	}

	t.Run("CreateAndDelete", func(t *testing.T) {
		// This test creates a number of hosts and deletes them immediately afterwards. If the delete operation for a
		// host is executed before the upsert operation, it would still exist in the database even though the object
		// is gone from Icinga 2.

		t.Parallel()

		namePrefix := utils.UniqueName(t, "host") + "-"

		for i := 0; i < numObjects; i++ {
			name := namePrefix + strconv.Itoa(i)
			client.CreateHost(t, name, nil)
			client.DeleteHost(t, name, true)
		}

		waitForPendingRuntimeUpdates(t)

		var countAfter int
		err = db.Get(&countAfter, "SELECT COUNT(*) FROM host WHERE name LIKE ?", namePrefix+"%")
		require.NoError(t, err, "select host count from database")
		assert.Zero(t, countAfter, "no hosts should be left in database")

	})

	t.Run("DeleteAndRecreate", func(t *testing.T) {
		// This test performs an operation that is probably more useful: delete an existing object and recreate it
		// immediately afterwards. If the delete operation is delayed, the host will be missing from the database.

		t.Parallel()

		namePrefix := utils.UniqueName(t, "host") + "-"
		hosts := make([]string, numObjects)

		for i := range hosts {
			name := namePrefix + strconv.Itoa(i)
			client.CreateHost(t, name, nil)
			hosts[i] = name
		}

		waitForPendingRuntimeUpdates(t)

		var countBefore int
		err = db.Get(&countBefore, "SELECT COUNT(*) FROM host WHERE name LIKE ?", namePrefix+"%")
		require.NoError(t, err, "select host count from database")
		assert.Equal(t, numObjects, countBefore, "all hosts should exist in database before recreation")

		for _, name := range hosts {
			client.DeleteHost(t, name, true)
			client.CreateHost(t, name, nil)
		}

		waitForPendingRuntimeUpdates(t)

		var countAfter int
		err = db.Get(&countAfter, "SELECT COUNT(*) FROM host WHERE name LIKE ?", namePrefix+"%")
		require.NoError(t, err, "select host count from database")
		assert.Equal(t, numObjects, countAfter, "all hosts should exist in database after recreation")
	})

}
