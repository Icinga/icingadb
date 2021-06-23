package types

import (
	"database/sql/driver"
	"github.com/icinga/icingadb/pkg/utils"
)

// PerfData is is used for columns of type mediumtext and stores performance date.
// Values greater than 65,535 bytes are truncated when they are written to the database.
type PerfData string

// Value implements driver.Valuer.
func (msg PerfData) Value() (driver.Value, error) {
	str, _ := utils.TruncatePerfData(string(msg), uint(16777215))

	return str, nil
}

// Assert interface compliance.
var (
	_ driver.Valuer = (*PerfData)(nil)
)
