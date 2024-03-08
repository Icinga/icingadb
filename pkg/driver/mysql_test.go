package driver

import (
	"context"
	"database/sql/driver"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"testing"
)

func TestSetGaleraOpts(t *testing.T) {
	tolerated := &mysql.MySQLError{
		Number:   errUnknownSysVar.Number,
		SQLState: [5]byte{255, 0, 42, 23, 7}, // Shall not confuse error comparison
		Message:  "This unusual text shall not confuse error comparison.",
	}

	almostTolerated := &mysql.MySQLError{}
	*almostTolerated = *tolerated
	almostTolerated.Number--

	notTolerated := io.EOF
	ignoredCodeLocation := os.ErrPermission

	subtests := []struct {
		name   string
		input  testConn
		output error
	}{{
		name:   "Conn PrepareContext returns error",
		input:  testConn{prepareError: notTolerated},
		output: notTolerated,
	}, {
		name:   "Conn PrepareContext returns MySQLError",
		input:  testConn{prepareError: almostTolerated},
		output: almostTolerated,
	}, {
		name:   "Conn PrepareContext returns MySQLError 1193",
		input:  testConn{prepareError: tolerated},
		output: nil,
	}, {
		name: "Stmt ExecContext returns error",
		input: testConn{preparedStmt: &testStmt{
			execError: notTolerated,
		}},
		output: notTolerated,
	}, {
		name: "Stmt ExecContext and Stmt Close return error",
		input: testConn{preparedStmt: &testStmt{
			execError:  notTolerated,
			closeError: ignoredCodeLocation,
		}},
		output: notTolerated,
	}, {
		name: "Stmt ExecContext returns MySQLError",
		input: testConn{preparedStmt: &testStmt{
			execError: almostTolerated,
		}},
		output: almostTolerated,
	}, {
		name: "Stmt ExecContext returns MySQLError and Stmt Close returns error",
		input: testConn{preparedStmt: &testStmt{
			execError:  almostTolerated,
			closeError: ignoredCodeLocation,
		}},
		output: almostTolerated,
	}, {
		name: "Stmt ExecContext returns MySQLError 1193",
		input: testConn{preparedStmt: &testStmt{
			execError: tolerated,
		}},
		output: nil,
	}, {
		name: "Stmt ExecContext and Stmt Close return MySQLError 1193",
		input: testConn{preparedStmt: &testStmt{
			execError:  tolerated,
			closeError: tolerated,
		}},
		output: tolerated,
	}, {
		name: "Stmt Close returns MySQLError 1193",
		input: testConn{preparedStmt: &testStmt{
			execResult: driver.ResultNoRows,
			closeError: tolerated,
		}},
		output: tolerated,
	}, {
		name: "no errors",
		input: testConn{preparedStmt: &testStmt{
			execResult: driver.ResultNoRows,
		}},
		output: nil,
	}}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			assert.ErrorIs(t, setGaleraOpts(context.Background(), &st.input, 7), st.output)
			assert.GreaterOrEqual(t, st.input.prepareCalls, uint8(1))

			if ts, ok := st.input.preparedStmt.(*testStmt); ok {
				assert.GreaterOrEqual(t, ts.execCalls, st.input.prepareCalls)
				assert.GreaterOrEqual(t, ts.closeCalls, st.input.prepareCalls)
			}
		})
	}
}

type testStmt struct {
	execResult driver.Result
	execError  error
	execCalls  uint8
	closeError error
	closeCalls uint8
}

// Close implements the driver.Stmt interface.
func (ts *testStmt) Close() error {
	ts.closeCalls++
	return ts.closeError
}

// NumInput implements the driver.Stmt interface.
func (*testStmt) NumInput() int {
	panic("don't call me")
}

// Exec implements the driver.Stmt interface.
func (*testStmt) Exec([]driver.Value) (driver.Result, error) {
	panic("don't call me")
}

// Query implements the driver.Stmt interface.
func (*testStmt) Query([]driver.Value) (driver.Rows, error) {
	panic("don't call me")
}

// ExecContext implements the driver.StmtExecContext interface.
func (ts *testStmt) ExecContext(context.Context, []driver.NamedValue) (driver.Result, error) {
	ts.execCalls++
	return ts.execResult, ts.execError
}

type testConn struct {
	preparedStmt driver.Stmt
	prepareError error
	prepareCalls uint8
}

// Prepare implements the driver.Conn interface.
func (*testConn) Prepare(string) (driver.Stmt, error) {
	panic("don't call me")
}

// Close implements the driver.Conn interface.
func (*testConn) Close() error {
	panic("don't call me")
}

// Begin implements the driver.Conn interface.
func (*testConn) Begin() (driver.Tx, error) {
	panic("don't call me")
}

// PrepareContext implements the driver.ConnPrepareContext interface.
func (tc *testConn) PrepareContext(context.Context, string) (driver.Stmt, error) {
	tc.prepareCalls++
	return tc.preparedStmt, tc.prepareError
}

// Assert interface compliance.
var (
	_ driver.Conn               = (*testConn)(nil)
	_ driver.ConnPrepareContext = (*testConn)(nil)
	_ driver.Stmt               = (*testStmt)(nil)
	_ driver.StmtExecContext    = (*testStmt)(nil)
)
