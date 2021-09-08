package icingadb_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"testing"
	"time"
)

func TestMultipleEnvironments(t *testing.T) {
	m := it.MysqlDatabaseT(t)
	m.ImportIcingaDbSchema()

	envs := []string{"", "some-env", "other-env"}

	var g errgroup.Group
	for _, env := range envs {
		env := env

		g.Go(func() error {
			r := it.RedisServerT(t)
			i := it.Icinga2NodeT(t, "master")
			conf := bytes.NewBuffer(nil)
			_, _ = fmt.Fprintf(conf, "const Environment = %q\n", env)
			i.WriteConfig("etc/icinga2/conf.d/testdata.conf", conf.Bytes())
			i.EnableIcingaDb(r)
			i.Reload()
			it.IcingaDbInstanceT(t, r, m)
			return nil
		})
	}
	_ = g.Wait()

	db, err := m.Open()
	require.NoError(t, err, "mysql open")
	t.Cleanup(func() { _ = db.Close() })

	t.Run("Table", func(t *testing.T) {
		t.Parallel()

		// TODO(jb): needs fix for issue #292
		t.Skip("environment table is currently not populated")

		assert.Eventually(t, func() bool {
			expectedRows := make(map[string]struct{})
			for _, env := range envs {
				expectedRows[env] = struct{}{}
			}

			rows, err := db.Query("SELECT id, name FROM environment")
			require.NoError(t, err, "mysql query")
			defer rows.Close()

			for rows.Next() {
				var id, name string
				err := rows.Scan(&id, &name)
				require.NoError(t, err, "mysql scan")

				if _, ok := expectedRows[name]; ok {
					delete(expectedRows, name)
				} else {
					return false
				}
			}

			return len(expectedRows) == 0
		}, 20*time.Second, 1*time.Second, "there should be one row in the environments tables for each one")
	})

	t.Run("HA", func(t *testing.T) {
		t.Parallel()

		assert.Eventually(t, func() bool {
			rows, err := db.Query("SELECT environment_id, COUNT(*) FROM icingadb_instance WHERE responsible = 'y' GROUP BY environment_id")
			require.NoError(t, err, "mysql query")
			defer rows.Close()

			numRows := 0
			for rows.Next() {
				var env []byte
				var count int
				err := rows.Scan(&env, &count)
				require.NoError(t, err, "mysql scan")

				assert.LessOrEqualf(t, count, 1,
					"environment %s must have at most one active instance", hex.EncodeToString(env))
				numRows++
			}
			return numRows == len(envs)
		}, 20*time.Second, 1*time.Second, "there should be one active instance per environment")
	})
}
