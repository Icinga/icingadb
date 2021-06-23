package types

import (
	"database/sql/driver"
	"github.com/icinga/icingadb/pkg/utils"
)

// Output is used for columns of type mediumtext.
// Values greater than 16,777,215 bytes are truncated such that last character remains a valid utf-8 character, when
// they are written to the database.
type Output string

// Value implements driver.Valuer.
func (msg Output) Value() (driver.Value, error) {
	str, _ := utils.TruncateText(string(msg), uint(16777215))

	return str, nil
}

// Assert interface compliance.
var (
	_ driver.Valuer = (*Output)(nil)
)
