// IcingaDB | (c) 2020 Icinga GmbH | GPLv2+

package connection

import (
	"database/sql"
	"github.com/stretchr/testify/assert"
	"testing"
)

func withDummyDb(t *testing.T, driver string, do func(db *sql.DB)) {
	t.Helper()

	db, errOp := sql.Open(driver, "/")
	if errOp != nil {
		t.Fatal(errOp)
	}

	defer db.Close()

	do(db)
}

func TestIsPostgres(t *testing.T) {
	withDummyDb(t, "mysql", func(db *sql.DB) {
		assert.Equal(t, false, IsPostgres(db))
	})

	withDummyDb(t, "postgres", func(db *sql.DB) {
		assert.Equal(t, true, IsPostgres(db))
	})

}

func TestPostgresPlaceholder(t *testing.T) {
	assert.Equal(t, "$1", postgresPlaceholder(0))
	assert.Equal(t, "$2", postgresPlaceholder(1))
	assert.Equal(t, "$3", postgresPlaceholder(2))
}

func TestPlaceholders(t *testing.T) {
	withDummyDb(t, "mysql", func(db *sql.DB) {
		assert.Equal(t, "?", Placeholders(db, 0, 1))
		assert.Equal(t, "?,?", Placeholders(db, 1, 2))
		assert.Equal(t, "?,?,?", Placeholders(db, 3, 3))
	})

	withDummyDb(t, "postgres", func(db *sql.DB) {
		assert.Equal(t, "$1", Placeholders(db, 0, 1))
		assert.Equal(t, "$2,$3", Placeholders(db, 1, 2))
		assert.Equal(t, "$4,$5,$6", Placeholders(db, 3, 3))
	})
}

func TestInsert(t *testing.T) {
	withDummyDb(t, "mysql", func(db *sql.DB) {
		assert.Equal(t, "INSERT INTO `t`(`a`,`b`)VALUES(?,?)", Insert(db, "t", "a", "b"))
	})

	withDummyDb(t, "postgres", func(db *sql.DB) {
		assert.Equal(t, `INSERT INTO "t"("a","b")VALUES($1,$2)`, Insert(db, "t", "a", "b"))
	})
}

func TestReplace(t *testing.T) {
	withDummyDb(t, "mysql", func(db *sql.DB) {
		assert.Equal(t, "REPLACE INTO `t`(`a`,`b`)VALUES(?,?)", Replace(db, "t", "a", "b"))
	})

	withDummyDb(t, "postgres", func(db *sql.DB) {
		assert.Equal(
			t, `INSERT INTO "t"("a","b")VALUES($1,$2)ON CONFLICT ON CONSTRAINT pk_t DO UPDATE SET "a"=$1,"b"=$2`,
			Replace(db, "t", "a", "b"),
		)
	})
}

func TestReplaceSomeIfZero(t *testing.T) {
	withDummyDb(t, "mysql", func(db *sql.DB) {
		assert.Equal(
			t,
			"INSERT INTO `t`(`a`,`b`,`c`,`d`)VALUES(?,?,?,?)"+
				"ON DUPLICATE KEY UPDATE `a`=IFNULL(NULLIF(`a`,0),VALUES(`a`)),`b`=IFNULL(NULLIF(`b`,''),VALUES(`b`))",
			ReplaceSomeIfZero(db, "t", []ReplacableColumn{{"a", "0"}, {"b", "''"}}, "c", "d"),
		)
	})

	withDummyDb(t, "postgres", func(db *sql.DB) {
		assert.Equal(
			t,
			`INSERT INTO "t"("a","b","c","d")VALUES($1,$2,$3,$4)ON CONFLICT ON CONSTRAINT pk_t`+
				` DO UPDATE SET "a"=COALESCE(NULLIF("a",0),$1),"b"=COALESCE(NULLIF("b",''),$2)`,
			ReplaceSomeIfZero(db, "t", []ReplacableColumn{{"a", "0"}, {"b", "''"}}, "c", "d"),
		)
	})
}

func TestUpdate(t *testing.T) {
	withDummyDb(t, "mysql", func(db *sql.DB) {
		assert.Equal(
			t, "UPDATE `t` SET `a`=?,`b`=? WHERE `c`=? AND `d`=?",
			Update(db, "t", []string{"c", "d"}, "a", "b"),
		)
	})

	withDummyDb(t, "postgres", func(db *sql.DB) {
		assert.Equal(
			t, `UPDATE "t" SET "a"=$1,"b"=$2 WHERE "c"=$3 AND "d"=$4`,
			Update(db, "t", []string{"c", "d"}, "a", "b"))
	})
}
