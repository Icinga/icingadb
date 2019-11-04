// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package mysqld

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var cComment = regexp.MustCompile(`/\*.*?\*/`)

func MkTestDb(host string) error {
	noDb, errNoDb := sql.Open("mysql", fmt.Sprintf("root@%s/", host))
	if errNoDb != nil {
		return errNoDb
	}

	defer noDb.Close()

	if _, errEx := noDb.Exec("CREATE DATABASE icingadb"); errEx != nil {
		return errEx
	}

	db, errDb := sql.Open("mysql", fmt.Sprintf("icingadb:icingadb@%s/icingadb", host))
	if errDb != nil {
		return errDb
	}

	defer db.Close()

	_, thisFile, _, _ := runtime.Caller(0)
	schema := path.Join(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))), "etc/schema/mysql")

	entries, errRD := ioutil.ReadDir(schema)
	if errRD != nil {
		return errRD
	}

	for _, entry := range entries {
		if name := entry.Name(); strings.HasSuffix(name, ".sql") {
			ddls, errRF := ioutil.ReadFile(path.Join(schema, name))
			if errRF != nil {
				return errRF
			}

			for _, ddl := range bytes.Split(ddls, []byte{';'}) {
				if ddl = bytes.TrimSpace(cComment.ReplaceAll(ddl, nil)); len(ddl) > 0 {
					if _, errEx := db.Exec(string(ddl)); errEx != nil {
						return errEx
					}
				}
			}
		}
	}

	return nil
}
