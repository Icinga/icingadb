// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package connection

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"io/ioutil"
	oldlog "log"
	"reflect"
	"sort"
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
	db.SetMaxIdleConns(0)

	return db, nil
}

type dbTypeBridge interface {
	sql.Scanner
	Result() interface{}
}

type dbIntBridge struct {
	result interface{}
}

func (d *dbIntBridge) Scan(src interface{}) (err error) {
	baseScanner := sql.NullInt64{}
	err = baseScanner.Scan(src)

	if err == nil {
		if baseScanner.Valid {
			d.result = baseScanner.Int64
		} else {
			d.result = nil
		}
	}

	return
}

func (d *dbIntBridge) Result() interface{} {
	return d.result
}

type dbFloatBridge struct {
	result interface{}
}

func (d *dbFloatBridge) Scan(src interface{}) (err error) {
	baseScanner := sql.NullFloat64{}
	err = baseScanner.Scan(src)

	if err == nil {
		if baseScanner.Valid {
			d.result = baseScanner.Float64
		} else {
			d.result = nil
		}
	}

	return
}

func (d *dbFloatBridge) Result() interface{} {
	return d.result
}

type dbStringBridge struct {
	result interface{}
}

func (d *dbStringBridge) Scan(src interface{}) (err error) {
	baseScanner := sql.NullString{}
	err = baseScanner.Scan(src)

	if err == nil {
		if baseScanner.Valid {
			d.result = baseScanner.String
		} else {
			d.result = nil
		}
	}

	return
}

func (d *dbStringBridge) Result() interface{} {
	return d.result
}

type dbBytesBridge struct {
	result interface{}
}

func (d *dbBytesBridge) Scan(src interface{}) (err error) {
	baseScanner := sql.NullString{}
	err = baseScanner.Scan(src)

	if err == nil {
		if baseScanner.Valid {
			d.result = []byte(baseScanner.String)
		} else {
			d.result = nil
		}
	}

	return
}

func (d *dbBytesBridge) Result() interface{} {
	return d.result
}

var dbTypeBridgeFactories = map[string]func() dbTypeBridge{
	// MySQL
	"TINYINT": func() dbTypeBridge {
		return &dbIntBridge{}
	},
	"SMALLINT": func() dbTypeBridge {
		return &dbIntBridge{}
	},
	"INT": func() dbTypeBridge {
		return &dbIntBridge{}
	},
	"BIGINT": func() dbTypeBridge {
		return &dbIntBridge{}
	},
	"FLOAT": func() dbTypeBridge {
		return &dbFloatBridge{}
	},
	"CHAR": func() dbTypeBridge {
		return &dbStringBridge{}
	},
	"VARCHAR": func() dbTypeBridge {
		return &dbStringBridge{}
	},
	"ENUM": func() dbTypeBridge {
		return &dbStringBridge{}
	},
	"BINARY": func() dbTypeBridge {
		return &dbBytesBridge{}
	},

	// SQLite
	"INTEGER": func() dbTypeBridge {
		return &dbIntBridge{}
	},
	"REAL": func() dbTypeBridge {
		return &dbFloatBridge{}
	},
	"TEXT": func() dbTypeBridge {
		return &dbStringBridge{}
	},
	"BLOB": func() dbTypeBridge {
		return &dbBytesBridge{}
	},
	// SELECT 1 FROM ...
	"": func() dbTypeBridge {
		return &dbIntBridge{}
	},
}

type dbBrokenBridge struct {
	typ string
}

func (d *dbBrokenBridge) Scan(src interface{}) error {
	types := make([]string, len(dbTypeBridgeFactories))
	typeIdx := 0

	for typ := range dbTypeBridgeFactories {
		types[typeIdx] = typ
		typeIdx++
	}

	sort.Strings(types)

	return errors.New(fmt.Sprintf("bad column type %s, expected one of %s", d.typ, strings.Join(types, ", ")))
}

func (d *dbBrokenBridge) Result() interface{} {
	return nil
}

var prettyPrintedSqlReplacer = strings.NewReplacer("\n", " ", "\t", "")

type prettyPrintedSql struct {
	sql string
}

// String implements and interface from Stringer
func (p prettyPrintedSql) String() string {
	return strings.TrimSpace(prettyPrintedSqlReplacer.Replace(p.sql))
}

// MarshalText implements an interface from TextMarshaler
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

// MarshalText implements an interface from TextMarshaler
func (p prettyPrintedArgs) MarshalText() (text []byte, err error) {
	return []byte(p.String()), nil
}

type prettyPrintedRowsAffected struct {
	result sql.Result
}

// String implements and interface from Stringer
func (d prettyPrintedRowsAffected) String() string {
	if d.result != nil {
		rows, errRA := d.result.RowsAffected()
		if errRA == nil {
			return strconv.FormatInt(rows, 10)
		}
	}

	return "N/A"
}

// MarshalText implements an interface from TextMarshaler
func (d prettyPrintedRowsAffected) MarshalText() (text []byte, err error) {
	return []byte(d.String()), nil
}

type MysqlConnectionError struct {
	err string
}

func (e MysqlConnectionError) Error() string {
	return e.err
}

// Returns whether the given error signals serialization failure
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

func ConvertValueForDb(in interface{}) (interface{}, error) {
	switch value := in.(type) {
	case []byte:
	case string:
	case float64:
	case int64:
	case nil:
	case float32:
		return float64(value), nil
	case uint:
		return int64(value), nil
	case uint8:
		return int64(value), nil
	case uint16:
		return int64(value), nil
	case uint32:
		return int64(value), nil
	case uint64:
		return int64(value), nil
	case int:
		return int64(value), nil
	case int8:
		return int64(value), nil
	case int16:
		return int64(value), nil
	case int32:
		return int64(value), nil
	case bool:
		return yesNo[value], nil
	default:
		return nil, errors.New(fmt.Sprintf(
			"bad type %s, expected one of []byte, string, float{32,64}, {,u}int{,8,16,32,64}, bool, nil",
			reflect.TypeOf(in).Name(),
		))
	}

	return in, nil
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
