package icingadb_test

import (
	"github.com/icinga/icinga-testing"
	"github.com/icinga/icinga-testing/services"
	"github.com/icinga/icingadb/tests/internal/utils"
	"testing"
)

var it *icingatesting.IT

func TestMain(m *testing.M) {
	it = icingatesting.NewIT()
	defer it.Cleanup()

	m.Run()
}

func getDatabase(t testing.TB) services.RelationalDatabase {
	rdb := getEmptyDatabase(t)

	rdb.ImportIcingaDbSchema()

	return rdb
}

func getEmptyDatabase(t testing.TB) services.RelationalDatabase {
	return utils.GetDatabase(it, t)
}
