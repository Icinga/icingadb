package icingadb_test

import (
	"encoding/hex"
	"encoding/json"
	"github.com/icinga/icinga-testing/services"
	"github.com/icinga/icinga-testing/utils/eventually"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"sort"
	"testing"
	"time"
)

func TestMultipleEnvironments(t *testing.T) {
	rdb := getDatabase(t)

	numEnvs := 3
	icinga2Instances := make([]services.Icinga2, numEnvs)

	// Start numEnvs icinga2 instances with an icingadb instance each, all writing to the same SQL database.
	var g errgroup.Group
	for i := range icinga2Instances {
		i := i

		g.Go(func() error {
			r := it.RedisServerT(t)
			icinga2Instances[i] = it.Icinga2NodeT(t, "master")
			icinga2Instances[i].EnableIcingaDb(r)
			icinga2Instances[i].Reload()
			it.IcingaDbInstanceT(t, r, rdb)
			return nil
		})
	}
	_ = g.Wait()

	// Query the IcingaDB environment_id from each icinga2 instance.
	var expectedEnvs []string
	for _, instance := range icinga2Instances {
		res, err := instance.ApiClient().GetJson("/v1/objects/icingadbs")
		require.NoError(t, err, "requesting IcingaDB objects from API should succeed")
		var objects ObjectsIcingaDBsResponse
		err = json.NewDecoder(res.Body).Decode(&objects)
		require.NoError(t, err, "requesting IcingaDB objects from API should succeed")
		require.NotEmpty(t, objects.Results, "API response should return an IcingaDB object")
		expectedEnvs = append(expectedEnvs, objects.Results[0].Attrs.EnvironmentId)
	}

	sort.Strings(expectedEnvs)
	for i := 0; i < len(expectedEnvs)-1; i++ {
		require.NotEqual(t, expectedEnvs[i], expectedEnvs[i+1], "all environment IDs should be distinct")
	}

	db, err := sqlx.Open(rdb.Driver(), rdb.DSN())
	require.NoError(t, err, "SQL database open")
	t.Cleanup(func() { _ = db.Close() })

	t.Run("Table", func(t *testing.T) {
		t.Parallel()

		eventually.Assert(t, func(t require.TestingT) {
			var query string
			switch rdb.Driver() {
			case "mysql":
				query = "SELECT LOWER(HEX(id)), name FROM environment ORDER BY id"
			case "postgres":
				query = "SELECT LOWER(ENCODE(id, 'hex')), name FROM environment ORDER BY id"
			default:
				panic("unknown database driver")
			}
			rows, err := db.Query(query)
			require.NoError(t, err, "SQL query")
			defer rows.Close()

			var gotEnvs []string
			for rows.Next() {
				var id, name string
				err := rows.Scan(&id, &name)
				require.NoError(t, err, "SQL scan")
				require.Equal(t, id, name, "name should be initialized to the environment id")
				gotEnvs = append(gotEnvs, id)
			}

			require.Equal(t, expectedEnvs, gotEnvs, "each environment should be present in the environments table")
		}, 20*time.Second, 250*time.Millisecond)
	})

	t.Run("HA", func(t *testing.T) {
		t.Parallel()

		eventually.Assert(t, func(t require.TestingT) {
			rows, err := db.Query("SELECT environment_id, COUNT(*) FROM icingadb_instance WHERE responsible = 'y' GROUP BY environment_id")
			require.NoError(t, err, "SQL query")
			defer rows.Close()

			numRows := 0
			for rows.Next() {
				var env []byte
				var count int
				err := rows.Scan(&env, &count)
				require.NoError(t, err, "SQL scan")

				assert.LessOrEqualf(t, count, 1,
					"environment %s must have at most one active instance", hex.EncodeToString(env))
				numRows++
			}
			require.Equal(t, numEnvs, numRows, "each environment should have one active instance")
		}, 20*time.Second, 250*time.Millisecond)
	})
}

type ObjectsIcingaDBsResponse struct {
	Results []struct {
		Attrs struct {
			EnvironmentId string `json:"environment_id"`
		} `json:"attrs"`
	} `json:"results"`
}
