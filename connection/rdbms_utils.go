// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package connection

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	"math"
	"strconv"
	"strings"
)

var prettyPrintedSqlReplacer = strings.NewReplacer("\n", " ", "\t", "")

type prettyPrintedSql struct {
	sql string
}

// String implements and interface from Stringer.
func (p prettyPrintedSql) String() string {
	return strings.TrimSpace(prettyPrintedSqlReplacer.Replace(p.sql))
}

// MarshalText implements an interface from TextMarshaler.
func (p prettyPrintedSql) MarshalText() (text []byte, err error) {
	return []byte(p.String()), nil
}

type prettyPrintedArgs struct {
	args []interface{}
}

func (p *prettyPrintedArgs) String() string {
	res := "["

	for _, v := range p.args {
		if byteArray, isByteArray := v.([]byte); isByteArray {
			res = fmt.Sprintf("%s hex.DecodeString(\"%s\"),", res, hex.EncodeToString(byteArray))
		} else {
			res = fmt.Sprintf("%s %#v,", res, v)
		}
	}

	return res + " ]"
}

// MarshalText implements an interface from TextMarshaler.
func (p prettyPrintedArgs) MarshalText() (text []byte, err error) {
	return []byte(p.String()), nil
}

type prettyPrintedRowsAffected struct {
	result sql.Result
}

// String implements and interface from Stringer.
func (d prettyPrintedRowsAffected) String() string {
	if d.result != nil {
		rows, errRA := d.result.RowsAffected()
		if errRA == nil {
			return strconv.FormatInt(rows, 10)
		}
	}

	return "N/A"
}

// MarshalText implements an interface from TextMarshaler.
func (d prettyPrintedRowsAffected) MarshalText() (text []byte, err error) {
	return []byte(d.String()), nil
}

type DbConnectionError struct {
	err string
}

func (e DbConnectionError) Error() string {
	return e.err
}

// isSerializationFailure returns whether the given error signals serialization failure.
// https://dev.mysql.com/doc/refman/5.5/en/error-messages-server.html#error_er_lock_deadlock
// https://www.postgresql.org/docs/9.5/transaction-iso.html
func isSerializationFailure(e error) bool {
	switch err := e.(type) {
	case *mysql.MySQLError:
		switch err.Number {
		// Those are the error numbers for serialization failures, upon which we retry
		case 1205, 1213:
			return true
		}
	case *pq.Error:
		return err.Code == "40001"
	}

	return false
}

func formatLogQuery(query string) string {
	r := strings.NewReplacer("\n", " ", "\t", "")
	return strings.TrimSpace(r.Replace(query))
}

// Go bool -> DB bool
var yesNo = map[bool]string{
	true:  "y",
	false: "n",
}

func ConvertValueForDb(in interface{}) interface{} {
	switch value := in.(type) {
	case []byte, string, float64, int64, nil:
		return in
	case float32:
		return float64(value)
	case uint:
		return int64(value)
	case uint8:
		return int64(value)
	case uint16:
		return int64(value)
	case uint32:
		return int64(value)
	case uint64:
		return int64(value)
	case int:
		return int64(value)
	case int8:
		return int64(value)
	case int16:
		return int64(value)
	case int32:
		return int64(value)
	case bool:
		return yesNo[value]
	default:
		return fmt.Sprintf("%s", in)
	}
}

type BulkInsertStmt struct {
	Table  string
	Fields []string
}

func NewBulkInsertStmt(table string, fields []string) *BulkInsertStmt {
	return &BulkInsertStmt{
		Table:  table,
		Fields: fields,
	}
}

type BulkDeleteStmt struct {
	Table      string
	PrimaryKey string
}

func NewBulkDeleteStmt(table string, primaryKey string) *BulkDeleteStmt {
	return &BulkDeleteStmt{
		Table:      table,
		PrimaryKey: primaryKey,
	}
}

type BulkUpdateStmt struct {
	Table  string
	Fields []string
}

func NewBulkUpdateStmt(table string, fields []string) *BulkUpdateStmt {
	return &BulkUpdateStmt{
		Table:  table,
		Fields: fields,
	}
}

type Row interface {
	InsertValues() []interface{}
	UpdateValues() []interface{}
	GetId() string
	SetId(id string)
	GetFinalRows() ([]Row, error)
}

type RowFactory func() Row

func ChunkRows(rows []Row, size int) [][]Row {
	chunksLen := int(math.Ceil(float64(len(rows)) / float64(size)))
	chunks := make([][]Row, chunksLen)

	for i := 0; i < chunksLen; i++ {
		start := i * size;
		end := start + size
		if end > len(rows) {
			end = len(rows)
		}

		chunks[i] = rows[start:end]
	}

	return chunks
}
