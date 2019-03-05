package icingadb_connection

import (
	"context"
	"database/sql"
	"errors"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
	_, err := NewDBWrapper("mysql", "asdasd")
	assert.Error(t, err)
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
	mockTx.On("Commit").Return( nil).Once()
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

	<- done

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

	<- done

	assert.NoError(t, err)
	mockDb.AssertExpectations(t)
}

func TestDBWrapper_SqlTransaction(t *testing.T) {
	dbw, err := NewDBWrapper("mysql", "module-dev:icinga0815!@tcp(127.0.0.1:3306)/icingadb")
	assert.NoError(t, err, "Is the MySQL server running?")

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

	<- done

	assert.NoError(t, err)
	mockDb.AssertExpectations(t)
}

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
		_, err = dbw.SqlExec("test", "test")
		done <- true
	}()

	time.Sleep(time.Millisecond * 50)

	dbw.CompareAndSetConnected(true)
	dbw.ConnectionUpCondition.Broadcast()

	<- done

	assert.NoError(t, err)
	mockDb.AssertExpectations(t)
}

func TestDBWrapper_SqlExecQuiet(t *testing.T) {
	mockDb := new(DbMock)
	dbw := NewTestDBW(mockDb)

	mockDb.On("Exec", "test", []interface{}(nil)).Return(SqlResultMock{}, errors.New("whoops")).Once()
	mockDb.On("Exec", "test", []interface{}(nil)).Return(SqlResultMock{}, nil).Once()
	mockDb.On("Ping").Return(errors.New("whoops")).Once()

	var err error
	done := make(chan bool)

	dbw.CompareAndSetConnected(true)
	go func() {
		_, err = dbw.SqlExecQuiet("test", "test")
		done <- true
	}()

	time.Sleep(time.Millisecond * 50)

	dbw.CompareAndSetConnected(true)
	dbw.ConnectionUpCondition.Broadcast()

	<- done

	assert.NoError(t, err)
	mockDb.AssertExpectations(t)
}

func TestDBWrapper_SqlExecTx(t *testing.T) {
	mockDb := new(DbMock)
	dbw := NewTestDBW(mockDb)
	mockTx := new(TransactionMock)

	mockTx.On("Exec", "test", []interface{}(nil)).Return(SqlResultMock{}, errors.New("whoops")).Once()
	mockTx.On("Exec", "test", []interface{}(nil)).Return(SqlResultMock{}, nil).Once()
	mockDb.On("Ping").Return(errors.New("whoops")).Once()

	var err error
	done := make(chan bool)

	dbw.CompareAndSetConnected(true)
	go func() {
		_, err = dbw.SqlExecTx(mockTx, "test", "test")
		done <- true
	}()

	time.Sleep(time.Millisecond * 50)

	dbw.CompareAndSetConnected(true)
	dbw.ConnectionUpCondition.Broadcast()

	<- done

	assert.NoError(t, err)
	mockTx.AssertExpectations(t)
	mockDb.AssertExpectations(t)
}

func TestDBWrapper_SqlExecTxQuiet(t *testing.T) {
	mockDb := new(DbMock)
	dbw := NewTestDBW(mockDb)
	mockTx := new(TransactionMock)

	mockTx.On("Exec", "test", []interface{}(nil)).Return(SqlResultMock{}, errors.New("whoops")).Once()
	mockTx.On("Exec", "test", []interface{}(nil)).Return(SqlResultMock{}, nil).Once()
	mockDb.On("Ping").Return(errors.New("whoops")).Once()

	var err error
	done := make(chan bool)

	dbw.CompareAndSetConnected(true)
	go func() {
		_, err = dbw.SqlExecTxQuiet(mockTx, "test", "test")
		done <- true
	}()

	time.Sleep(time.Millisecond * 50)

	dbw.CompareAndSetConnected(true)
	dbw.ConnectionUpCondition.Broadcast()

	<- done

	assert.NoError(t, err)
	mockTx.AssertExpectations(t)
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

	//dbw.ConnectionLostCounter = 14
	//interval = dbw.getConnectionCheckInterval()
	//TODO: Check for Fatal
}
