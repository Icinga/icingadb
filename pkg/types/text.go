package types

import (
	"database/sql/driver"
	"github.com/icinga/icingadb/pkg/utils"
)

// Text is used for columns of type text.
// Values greater than 65,535 bytes are truncated such that last character remains a valid utf-8 character, when they
// are written to the database.
type Text string

// Value implements driver.Valuer.
func (msg Text) Value() (driver.Value, error) {
	str, _ := utils.TruncateText(string(msg), uint(65535))

	return str, nil
}

// Assert interface compliance.
var (
	_ driver.Valuer = (*Text)(nil)
)
