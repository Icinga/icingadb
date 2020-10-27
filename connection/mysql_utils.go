// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package connection

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"io/ioutil"
	oldlog "log"
	"math"
	"strconv"
	"strings"
)

// mkMysql creates a new MySQL client.
func mkMysql(dbType string, dbDsn string, maxOpenConns int) (*sql.DB, error) {
	sep := "?"

	if dbDsn == "" {
		dbDsn = "/"
	} else {
		dsnParts := strings.Split(dbDsn, "/")
		if strings.Contains(dsnParts[len(dsnParts)-1], "?") {
			sep = "&"
		}
	}

	dbDsn = dbDsn + sep +
		"innodb_strict_mode=1&sql_mode='STRICT_ALL_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,NO_ENGINE_SUBSTITUTION,PIPES_AS_CONCAT,ANSI_QUOTES,ERROR_FOR_DIVISION_BY_ZERO'"

	db, errConn := sql.Open(dbType, dbDsn)
	if errConn != nil {
		return nil, errConn
	}

	mysql.SetLogger(oldlog.New(ioutil.Discard, "", 0))

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxOpenConns)

	return db, nil
}

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

type MysqlConnectionError struct {
	err string
}

func (e MysqlConnectionError) Error() string {
	return e.err
}

// isSerializationFailure returns whether the given error signals serialization failure.
// https://dev.mysql.com/doc/refman/5.5/en/error-messages-server.html#error_er_lock_deadlock
func isSerializationFailure(e error) bool {
	switch err := e.(type) {
	case *mysql.MySQLError:
		switch err.Number {
		// Those are the error numbers for serialization failures, upon which we retry
		case 1205, 1213:
			return true
		}
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

func MakePlaceholderList(x int) string {
	runes := make([]rune, 1+x*2)

	i := 1
	for j := 0; j < x; j++ {
		runes[i] = '?'
		i++

		runes[i] = ','
		i++
	}

	runes[0] = '('
	runes[len(runes)-1] = ')'

	return string(runes)
}

func isRetryableError(err error) bool {
	if strings.Contains(err.Error(), "Deadlock found when trying to get lock") {
		return true
	}
	return false
}

type BulkInsertStmt struct {
	Format      string
	Fields      []string
	Placeholder string
	NumField    int
}

func NewBulkInsertStmt(table string, fields []string) *BulkInsertStmt {
	numField := len(fields)
	placeholder := fmt.Sprintf("(%s)", strings.TrimSuffix(strings.Repeat("?, ", numField), ", "))
	stmt := BulkInsertStmt{
		Format:      fmt.Sprintf("REPLACE INTO %s (%s) VALUES %s", table, strings.Join(fields, ", "), "%s"),
		Fields:      fields,
		Placeholder: placeholder,
		NumField:    numField,
	}

	return &stmt
}

type BulkDeleteStmt struct {
	Format string
}

func NewBulkDeleteStmt(table string, primaryKey string) *BulkDeleteStmt {
	stmt := BulkDeleteStmt{
		Format: fmt.Sprintf("DELETE FROM %s WHERE %s IN (%s)", table, primaryKey, "%s"),
	}

	return &stmt
}

type BulkUpdateStmt struct {
	Format      string
	Fields      []string
	Placeholder string
	NumField    int
}

func NewBulkUpdateStmt(table string, fields []string) *BulkUpdateStmt {
	numField := len(fields)
	placeholder := fmt.Sprintf("(%s)", strings.TrimSuffix(strings.Repeat("?, ", numField), ", "))
	stmt := BulkUpdateStmt{
		Format:      fmt.Sprintf("REPLACE INTO %s (%s) VALUES %s", table, strings.Join(fields, ", "), "%s"),
		Fields:      fields,
		Placeholder: placeholder,
		NumField:    numField,
	}

	return &stmt
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
