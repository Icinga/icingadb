package icingadb_test

import (
	"github.com/icinga/icinga-testing"
	"testing"
)

var it *icingatesting.IT

func TestMain(m *testing.M) {
	it = icingatesting.NewIT()
	defer it.Cleanup()

	m.Run()
}
