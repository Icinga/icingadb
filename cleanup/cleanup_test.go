package cleanup

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Icinga/icingadb/config/testbackends"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/utils"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

func NewTestCleanup(dbw *connection.DBWrapper) *Cleanup {
	return &Cleanup{
		dbw: dbw,
		wg:  &sync.WaitGroup{},
		tick: time.Hour,
		limit: 10,
	}
}

func TestCleanupMore(t *testing.T) {
	dbw, err := connection.NewDBWrapper(testbackends.MysqlTestDsn + "?allowAllFiles=true", 50)
	require.NoError(t, err, "Is the MySQL server running?")
	testclean := NewTestCleanup(dbw)

	startTime := time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC)

	_, err = dbw.Db.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS testing123 (data text NOT NULL, event_time bigint(20) NOT NULL, INDEX idx_event_time (event_time))`))
	require.NoError(t, err)

	filename := "testdatamore.csv"
	mysql.RegisterLocalFile(filename)

	result, err := dbw.Db.Exec(fmt.Sprintf(`LOAD DATA LOCAL INFILE '%s' INTO TABLE testing123 FIELDS TERMINATED BY ',' LINES TERMINATED BY '\n'`, filename))
	require.NoError(t, err)

	affected, err := result.RowsAffected()
	assert.NoError(t, err)

	assert.Equal(t, int64(200), affected)

	ctx := context.TODO()

	err, cnt := testclean.cleanup(ctx, Tableconfig{"testing123", time.Hour, startTime}, CleanTestingFunc)
	assert.NoError(t, err)

	rows, err := dbw.Db.Query("SELECT COUNT(*) FROM  testing123")
	assert.NoError(t, err)

	var count_product int64
	for rows.Next() {
		rows.Scan(&count_product)
	}

	assert.Equal(t, int64(100), count_product)
	assert.Equal(t, 11, cnt)
	_, err = dbw.Db.Exec("DROP TABLE testing123")
	assert.NoError(t, err)
}

func TestCleanupLess(t *testing.T) {
	dbw, err := connection.NewDBWrapper(testbackends.MysqlTestDsn + "?allowAllFiles=true", 50)
	require.NoError(t, err, "Is the MySQL server running?")
	testclean := NewTestCleanup(dbw)

	startTime := time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC)

	_, err = dbw.Db.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS testing123 (data text NOT NULL, event_time bigint(20) NOT NULL, INDEX idx_event_time (event_time))`))
	require.NoError(t, err)

	filename := "testdataless.csv"
	mysql.RegisterLocalFile(filename)
	result, err := dbw.Db.Exec(fmt.Sprintf(`LOAD DATA LOCAL INFILE '%s' INTO TABLE testing123 FIELDS TERMINATED BY ',' LINES TERMINATED BY '\n'`, filename))
	require.NoError(t, err)

	affected, err := result.RowsAffected()
	assert.NoError(t, err)

	assert.Equal(t, int64(8), affected)
	ctx := context.TODO()

	err, cnt := testclean.cleanup(ctx, Tableconfig{"testing123", time.Hour, startTime}, CleanTestingFunc)
	assert.NoError(t, err)

	rows, err := dbw.Db.Query("SELECT COUNT(*) FROM  testing123")
	assert.NoError(t, err)

	var count_product int64
	for rows.Next() {
		rows.Scan(&count_product)
	}

	assert.Equal(t, int64(0), count_product)
	assert.Equal(t, 1, cnt)
	_, err = dbw.Db.Exec("DROP TABLE testing123")
	assert.NoError(t, err)
}

func TestCleanupEqual(t *testing.T) {
	dbw, err := connection.NewDBWrapper(testbackends.MysqlTestDsn + "?allowAllFiles=true", 50)
	require.NoError(t, err, "Is the MySQL server running?")
	testclean := NewTestCleanup(dbw)

	startTime := time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC)

	_, err = dbw.Db.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS testing123 (data text NOT NULL, event_time bigint(20) NOT NULL, INDEX idx_event_time (event_time))`))
	require.NoError(t, err)

	filename := "testdataequal.csv"
	mysql.RegisterLocalFile(filename)

	result, err := dbw.Db.Exec(fmt.Sprintf(`LOAD DATA LOCAL INFILE '%s' INTO TABLE testing123 FIELDS TERMINATED BY ',' LINES TERMINATED BY '\n'`, filename))
	require.NoError(t, err)

	affected, err := result.RowsAffected()
	assert.NoError(t, err)

	assert.Equal(t, int64(100), affected)
	ctx := context.TODO()

	err, cnt := testclean.cleanup(ctx, Tableconfig{"testing123", time.Hour, startTime}, CleanTestingFunc)
	assert.NoError(t, err)

	rows, err := dbw.Db.Query("SELECT COUNT(*) FROM  testing123")
	assert.NoError(t, err)

	var count_product int64
	for rows.Next() {
		rows.Scan(&count_product)
	}

	assert.Equal(t, int64(0), count_product)
	assert.Equal(t, 11, cnt)
	_, err = dbw.Db.Exec("DROP TABLE testing123")
	assert.NoError(t, err)
}

func CleanTestingFunc(ctx context.Context, tblcfg Tableconfig, db *connection.DBWrapper, limit int) (sql.Result, error){
	event_time := utils.TimeToMillisecs(tblcfg.Starttime.Add(-1*tblcfg.Period))
	result, err := db.Db.ExecContext(
		ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE %s.event_time < ? LIMIT %d`, tblcfg.Table, tblcfg.Table, limit),
		event_time)

	return result, err
}
