// IcingaDB | (c) 2020 Icinga GmbH | GPLv2+

package connection

import (
	"github.com/lib/pq"
	"strconv"
	"strings"
)

// postgresPlaceholder returns "$1" if i==0.
func postgresPlaceholder(i int) string {
	return "$" + strconv.FormatInt(int64(i+1), 10)
}

type sqlToolbox struct {
	placeholder func(int) string
	escapeName  func(string) string
}

var mysqlToolbox = sqlToolbox{
	func(int) string {
		return "?"
	},
	func(name string) string {
		return "`" + name + "`"
	},
}

var postgresToolbox = sqlToolbox{
	postgresPlaceholder,
	func(name string) string {
		return `"` + name + `"`
	},
}

func IsPostgres(db DbClient) bool {
	switch db.Driver().(type) {
	case *pq.Driver:
		return true
	default:
		return false
	}
}

func getSqlToolbox(db DbClient) *sqlToolbox {
	if IsPostgres(db) {
		return &postgresToolbox
	} else {
		return &mysqlToolbox
	}
}

// Placeholders returns amount placeholders separated by comma suitable for db.
// Specify in start how many placeholders are there in the target query before these ones.
func Placeholders(db DbClient, start, amount int) string {
	toolbox := getSqlToolbox(db)
	placeholders := make([]string, 0, amount)

	for i := 0; i < amount; i++ {
		placeholders = append(placeholders, toolbox.placeholder(start+i))
	}

	return strings.Join(placeholders, ",")
}

func EscapeName(db DbClient, name string) string {
	return getSqlToolbox(db).escapeName(name)
}

// Insert assembles an SQL INSERT statement for columns of table suitable for db.
func Insert(db DbClient, table string, columns ...string) string {
	toolbox := getSqlToolbox(db)
	columns = append([]string(nil), columns...)
	placeholders := make([]string, 0, len(columns))

	for i, name := range columns {
		columns[i] = toolbox.escapeName(name)
		placeholders = append(placeholders, toolbox.placeholder(i))
	}

	return "INSERT INTO " + toolbox.escapeName(table) + "(" + strings.Join(columns, ",") +
		")VALUES(" + strings.Join(placeholders, ",") + ")"
}

// Replace assembles an SQL "REPLACE INTO ..." / "INSERT INTO ... ON CONFLICT DO UPDATE ..." statement
// for columns of table suitable for db.
func Replace(db DbClient, table string, columns ...string) string {
	toolbox := getSqlToolbox(db)
	columns = append([]string(nil), columns...)
	placeholders := make([]string, 0, len(columns))

	for i, name := range columns {
		columns[i] = toolbox.escapeName(name)
		placeholders = append(placeholders, toolbox.placeholder(i))
	}

	if toolbox == &postgresToolbox {
		update := make([]string, 0, len(columns))
		for i, name := range columns {
			update = append(update, name+"="+toolbox.placeholder(i))
		}

		return "INSERT INTO " + toolbox.escapeName(table) + "(" + strings.Join(columns, ",") +
			")VALUES(" + strings.Join(placeholders, ",") + ")ON CONFLICT ON CONSTRAINT pk_" +
			table + " DO UPDATE SET " + strings.Join(update, ",")
	} else {
		return "REPLACE INTO " + toolbox.escapeName(table) + "(" + strings.Join(columns, ",") +
			")VALUES(" + strings.Join(placeholders, ",") + ")"
	}
}

// ReplacableColumn represents a column to be updated by ReplaceSomeIfZero.
type ReplacableColumn struct {
	// Name specifies the (unescaped) column name.
	Name string
	// ZeroValue specifies the (ANSI SQL) value to consider zero.
	ZeroValue string
}

// ReplaceSomeIfZero assembles an SQL INSERT statement with "ON DUPLICATE KEY UPDATE" / "ON CONFLICT DO UPDATE"
// for replacableColumns and otherColumns of table suitable for db.
// On conflict only replacableColumns are updated and only if they're zero or NULL.
func ReplaceSomeIfZero(db DbClient, table string, replacableColumns []ReplacableColumn, otherColumns ...string) string {
	toolbox := getSqlToolbox(db)
	columns := make([]string, 0, len(replacableColumns)+len(otherColumns))
	placeholders := make([]string, 0, cap(columns))

	for _, column := range replacableColumns {
		columns = append(columns, column.Name)
	}

	columns = append(columns, otherColumns...)

	for i, name := range columns {
		columns[i] = toolbox.escapeName(name)
		placeholders = append(placeholders, toolbox.placeholder(i))
	}

	var onConflict string
	update := make([]string, 0, len(replacableColumns))

	if toolbox == &postgresToolbox {
		onConflict = "ON CONFLICT ON CONSTRAINT pk_" + table + " DO UPDATE SET"
		for i, column := range replacableColumns {
			update = append(
				update,
				columns[i]+"=COALESCE(NULLIF("+columns[i]+","+column.ZeroValue+"),"+toolbox.placeholder(i)+")",
			)
		}
	} else {
		onConflict = "ON DUPLICATE KEY UPDATE"
		for i, column := range replacableColumns {
			update = append(
				update,
				columns[i]+"=IFNULL(NULLIF("+columns[i]+","+column.ZeroValue+"),VALUES("+columns[i]+"))",
			)
		}
	}

	return "INSERT INTO " + toolbox.escapeName(table) + "(" + strings.Join(columns, ",") + ")VALUES(" +
		strings.Join(placeholders, ",") + ")" + onConflict + " " + strings.Join(update, ",")
}

// Update assembles an SQL UPDATE statement for whereColumns and setColumns of table suitable for db.
func Update(db DbClient, table string, whereColumns []string, setColumns ...string) string {
	toolbox := getSqlToolbox(db)
	columns := append(append([]string(nil), setColumns...), whereColumns...)

	for i, name := range columns {
		columns[i] = toolbox.escapeName(name) + "=" + toolbox.placeholder(i)
	}

	return "UPDATE " + toolbox.escapeName(table) + " SET " + strings.Join(columns[:len(setColumns)], ",") +
		" WHERE " + strings.Join(columns[len(setColumns):], " AND ")
}
