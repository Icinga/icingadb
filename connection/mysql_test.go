// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package connection

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"errors"
	"github.com/Icinga/icingadb/config/testbackends"
	"github.com/Icinga/icingadb/utils"
	"github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type SqlResultMock struct {
	sql.Result
}

type TransactionMock struct {
	mock.Mock
}

func (m *TransactionMock) Query(query string, args ...interface{}) (*sql.Rows, error) {
	args2 := m.Called(query, args)
	return args2.Get(0).(*sql.Rows), args2.Error(1)
}

func (m *TransactionMock) Exec(query string, args ...interface{}) (sql.Result, error) {
	args2 := m.Called(query, args)
	return args2.Get(0).(sql.Result), args2.Error(1)
}

func (m *TransactionMock) Commit() error {
	args := m.Called()
	return args.Error(0)
}

func (m *TransactionMock) Rollback() error {
	args := m.Called()
	return args.Error(0)
}

type DbMock struct {
	mock.Mock
}

func (m *DbMock) Ping() error {
	args := m.Called()
	return args.Error(0)
}

func (m *DbMock) Query(query string, args ...interface{}) (*sql.Rows, error) {
	args2 := m.Called(query, args)
	return args2.Get(0).(*sql.Rows), args2.Error(1)
}

func (m *DbMock) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	args := m.Called(ctx, opts)
	return args.Get(0).(*sql.Tx), args.Error(1)
}

func (m *DbMock) Exec(query string, args ...interface{}) (sql.Result, error) {
	args2 := m.Called(query, args)
	return args2.Get(0).(sql.Result), args2.Error(1)
}

func NewTestDBW(db DbClient) DBWrapper {
	dbw := DBWrapper{Db: db, ConnectedAtomic: new(uint32), ConnectionLostCounterAtomic: new(uint32)}
	dbw.ConnectionUpCondition = sync.NewCond(&sync.Mutex{})
	return dbw
}

func TestNewDBWrapper(t *testing.T) {
	dbw, err := NewDBWrapper("asdasd", 50)
	if err == nil {
		assert.False(t, dbw.checkConnection(false), "DBWrapper should not be connected")
	}

	//TODO: Add more tests here
}

func TestDBWrapper_CheckConnection(t *testing.T) {
	mockDb := new(DbMock)
	dbw := NewTestDBW(mockDb)

	atomic.StoreUint32(dbw.ConnectionLostCounterAtomic, 512312312)
	mockDb.On("Ping").Return(nil).Once()
	assert.True(t, dbw.checkConnection(false), "DBWrapper should be connected")
	assert.Equal(t, uint32(0), atomic.LoadUint32(dbw.ConnectionLostCounterAtomic))

	atomic.StoreUint32(dbw.ConnectionLostCounterAtomic, 0)
	mockDb.On("Ping").Return(mysql.ErrInvalidConn).Once()
	assert.False(t, dbw.checkConnection(false), "DBWrapper should not be connected")
	assert.Equal(t, uint32(0), atomic.LoadUint32(dbw.ConnectionLostCounterAtomic))

	atomic.StoreUint32(dbw.ConnectionLostCounterAtomic, 10)
	mockDb.On("Ping").Return(mysql.ErrInvalidConn).Once()
	assert.False(t, dbw.checkConnection(true), "DBWrapper should not be connected")
	assert.Equal(t, uint32(11), atomic.LoadUint32(dbw.ConnectionLostCounterAtomic))
}

func TestDBWrapper_SqlCommit(t *testing.T) {
	mockDb := new(DbMock)
	dbw := NewTestDBW(mockDb)
	mockTx := new(TransactionMock)

	mockTx.On("Commit").Return(errors.New("whoops")).Once()
	mockTx.On("Commit").Return(nil).Once()
	mockDb.On("Ping").Return(errors.New("whoops")).Once()

	var err error
	done := make(chan bool)

	dbw.CompareAndSetConnected(true)
	go func() {
		err = dbw.SqlCommit(mockTx, false)
		done <- true
	}()

	time.Sleep(time.Millisecond * 50)

	dbw.CompareAndSetConnected(true)
	dbw.ConnectionUpCondition.Broadcast()

	<-done

	assert.NoError(t, err)
	mockTx.AssertExpectations(t)
	mockDb.AssertExpectations(t)
}

func TestDBWrapper_SqlBegin(t *testing.T) {
	mockDb := new(DbMock)
	dbw := NewTestDBW(mockDb)

	mockDb.On("BeginTx", context.Background(), &sql.TxOptions{Isolation: sql.LevelReadCommitted}).Return(&sql.Tx{}, errors.New("whoops")).Once()
	mockDb.On("BeginTx", context.Background(), &sql.TxOptions{Isolation: sql.LevelReadCommitted}).Return(&sql.Tx{}, nil).Once()
	mockDb.On("Ping").Return(errors.New("whoops")).Once()

	var err error
	done := make(chan bool)

	dbw.CompareAndSetConnected(true)
	go func() {
		_, err = dbw.SqlBegin(false, false)
		done <- true
	}()

	time.Sleep(time.Millisecond * 50)

	dbw.CompareAndSetConnected(true)
	dbw.ConnectionUpCondition.Broadcast()

	<-done

	assert.NoError(t, err)
	mockDb.AssertExpectations(t)
}

func TestDBWrapper_SqlTransaction(t *testing.T) {
	dbw, err := NewDBWrapper(testbackends.MysqlTestDsn, 50)
	require.NoError(t, err, "Is the MySQL server running?")

	err = dbw.SqlTransaction(false, true, false, func(tx DbTransaction) error {
		return nil
	})

	assert.NoError(t, err)

	err = dbw.SqlTransaction(false, true, false, func(tx DbTransaction) error {
		return errors.New("whoops")
	})

	assert.Error(t, err)
}

func TestDBWrapper_WithRetry(t *testing.T) {
	mockDb := new(DbMock)
	dbw := NewTestDBW(mockDb)

	tries := 0

	_, err := dbw.WithRetry(func() (result sql.Result, e error) {
		if tries > 0 {
			tries++
			return nil, nil
		} else {
			tries++
			return nil, errors.New("Deadlock found when trying to get lock")
		}
	})

	assert.NoError(t, err)
	assert.Equal(t, 2, tries)

	_, err = dbw.WithRetry(func() (result sql.Result, e error) {
		return nil, errors.New("something went wrong")
	})

	assert.Error(t, err)
}

func TestDBWrapper_SqlQuery(t *testing.T) {
	mockDb := new(DbMock)
	dbw := NewTestDBW(mockDb)

	mockDb.On("Query", "test", []interface{}(nil)).Return(&sql.Rows{}, errors.New("whoops")).Once()
	mockDb.On("Query", "test", []interface{}(nil)).Return(&sql.Rows{}, nil).Once()
	mockDb.On("Ping").Return(errors.New("whoops")).Once()

	var err error
	done := make(chan bool)

	dbw.CompareAndSetConnected(true)
	go func() {
		_, err = dbw.SqlQuery("test")
		done <- true
	}()

	time.Sleep(time.Millisecond * 50)

	dbw.CompareAndSetConnected(true)
	dbw.ConnectionUpCondition.Broadcast()

	<-done

	assert.NoError(t, err)
	mockDb.AssertExpectations(t)
}

var mysqlTestObserver = DbIoSeconds.WithLabelValues("mysql", "test")

func TestDBWrapper_SqlExec(t *testing.T) {
	mockDb := new(DbMock)
	dbw := NewTestDBW(mockDb)

	mockDb.On("Exec", "test", []interface{}(nil)).Return(SqlResultMock{}, errors.New("whoops")).Once()
	mockDb.On("Exec", "test", []interface{}(nil)).Return(SqlResultMock{}, nil).Once()
	mockDb.On("Ping").Return(errors.New("whoops")).Once()

	var err error
	done := make(chan bool)

	dbw.CompareAndSetConnected(true)
	go func() {
		_, err = dbw.SqlExec(mysqlTestObserver, "test")
		done <- true
	}()

	time.Sleep(time.Millisecond * 50)

	dbw.CompareAndSetConnected(true)
	dbw.ConnectionUpCondition.Broadcast()

	<-done

	assert.NoError(t, err)
	mockDb.AssertExpectations(t)
}

func TestGetConnectionCheckInterval(t *testing.T) {
	dbw := NewTestDBW(nil)

	//Should return 15s, if connected - counter doesn't madder
	dbw.CompareAndSetConnected(true)
	assert.Equal(t, 15*time.Second, dbw.getConnectionCheckInterval())

	//Should return 5s, if not connected and counter < 4
	dbw.CompareAndSetConnected(false)
	atomic.StoreUint32(dbw.ConnectionLostCounterAtomic, 0)
	assert.Equal(t, 5*time.Second, dbw.getConnectionCheckInterval())

	//Should return 10s, if not connected and 4 <= counter < 8
	dbw.CompareAndSetConnected(false)
	atomic.StoreUint32(dbw.ConnectionLostCounterAtomic, 4)
	assert.Equal(t, 10*time.Second, dbw.getConnectionCheckInterval())

	//Should return 30s, if not connected and 8 <= counter < 11
	dbw.CompareAndSetConnected(false)
	atomic.StoreUint32(dbw.ConnectionLostCounterAtomic, 8)
	assert.Equal(t, 30*time.Second, dbw.getConnectionCheckInterval())

	//Should return 60s, if not connected and 11 <= counter < 14
	dbw.CompareAndSetConnected(false)
	atomic.StoreUint32(dbw.ConnectionLostCounterAtomic, 11)
	assert.Equal(t, 60*time.Second, dbw.getConnectionCheckInterval())

	//Should exit, if not connected and counter > 13
	dbw.CompareAndSetConnected(false)
	atomic.StoreUint32(dbw.ConnectionLostCounterAtomic, 14)

	exited := false
	defer func() { logrus.StandardLogger().ExitFunc = nil }()
	logrus.StandardLogger().ExitFunc = func(i int) {
		exited = true
	}

	dbw.getConnectionCheckInterval()
	assert.Equal(t, true, exited, "Should have exited")
}

func TestDBWrapper_SqlFetchAll(t *testing.T) {
	type row struct {
		Id   int32
		Name string
	}

	dbw, err := NewDBWrapper(testbackends.MysqlTestDsn, 50)
	require.NoError(t, err, "Is the MySQL server running?")

	_, err = dbw.Db.Exec("CREATE TABLE testing0815 (id INT NOT NULL AUTO_INCREMENT PRIMARY KEY, name varchar(255) NOT NULL)")
	require.NoError(t, err)

	_, err = dbw.Db.Exec("INSERT INTO testing0815 (name) VALUES ('horst'), ('test')")
	require.NoError(t, err)

	var res interface{}
	done := make(chan bool)
	dbw.CompareAndSetConnected(false)
	go func() {
		res, err = dbw.SqlFetchAll(mysqlTestObserver, row{}, "SELECT id, name FROM testing0815")
		done <- true
	}()

	time.Sleep(time.Millisecond * 50)

	dbw.checkConnection(true)

	<-done

	assert.NoError(t, err)
	assert.Equal(t, []row{{1, "horst"}, {2, "test"}}, res)

	_, err = dbw.Db.Exec("DROP TABLE testing0815")
	assert.NoError(t, err)
}

func TestDBWrapper_SqlFetchIds(t *testing.T) {
	dbw, err := NewDBWrapper(testbackends.MysqlTestDsn, 50)
	require.NoError(t, err, "Is the MySQL server running?")

	hash := sha1.New()
	hash.Write([]byte("derp"))
	envId := hash.Sum(nil)

	_, err = dbw.Db.Exec("CREATE TABLE testing0815 (id binary(20) NOT NULL PRIMARY KEY, environment_id binary(20) NOT NULL)")
	assert.NoError(t, err)

	hashHorst := sha1.New()
	hashHorst.Write([]byte("horst"))
	horst := hashHorst.Sum(nil)

	hashPeter := sha1.New()
	hashPeter.Write([]byte("peter"))
	peter := hashPeter.Sum(nil)

	_, err = dbw.Db.Exec("INSERT INTO testing0815 (id, environment_id) VALUES (?, ?), (?, ?)", horst, envId, peter, envId)
	assert.NoError(t, err)

	ids, err := dbw.SqlFetchIds(envId, "testing0815", "id")
	assert.NoError(t, err)

	assert.ElementsMatch(t, []string{utils.DecodeChecksum(horst), utils.DecodeChecksum(peter)}, ids)

	_, err = dbw.Db.Exec("DROP TABLE testing0815")
	assert.NoError(t, err)
}

func TestDBWrapper_SqlFetchChecksums(t *testing.T) {
	dbw, err := NewDBWrapper(testbackends.MysqlTestDsn, 50)
	require.NoError(t, err, "Is the MySQL server running?")

	envId := utils.Checksum("derp")

	_, err = dbw.Db.Exec("CREATE TABLE testing0815 (id binary(20) NOT NULL PRIMARY KEY, environment_id binary(20) NOT NULL, properties_checksum binary(20) NOT NULL)")
	assert.NoError(t, err)

	horst := utils.Checksum("horst")
	peter := utils.Checksum("peter")

	_, err = dbw.Db.Exec("INSERT INTO testing0815 (id, environment_id, properties_checksum) VALUES (?, ?, ?), (?, ?, ?)", utils.EncodeChecksum(horst), utils.EncodeChecksum(envId), utils.EncodeChecksum(utils.Checksum("hans wurst")), utils.EncodeChecksum(peter), utils.EncodeChecksum(envId), utils.EncodeChecksum(utils.Checksum("peter wurst")))
	assert.NoError(t, err)

	checksums, err := dbw.SqlFetchChecksums("testing0815", []string{horst, peter})
	assert.NoError(t, err)

	assert.Equal(t, utils.Checksum("hans wurst"), checksums[horst]["properties_checksum"])
	assert.Equal(t, utils.Checksum("peter wurst"), checksums[peter]["properties_checksum"])

	_, err = dbw.Db.Exec("DROP TABLE testing0815")
	assert.NoError(t, err)
}
