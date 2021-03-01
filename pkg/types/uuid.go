package types

import (
	"database/sql/driver"
	"encoding"
	"github.com/google/uuid"
)

// UUID is like uuid.UUID, but marshals itself binarily (not like xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx) in SQL context.
type UUID struct {
	uuid.UUID
}

// Value implements driver.Valuer.
func (uuid UUID) Value() (driver.Value, error) {
	return uuid.UUID[:], nil
}

// Assert interface compliance.
var (
	_ encoding.TextUnmarshaler = (*UUID)(nil)
	_ driver.Valuer            = UUID{}
	_ driver.Valuer            = (*UUID)(nil)
)
