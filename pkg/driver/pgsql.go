package driver

import (
	"database/sql/driver"
	"github.com/lib/pq"
)

// PgSQLDriver extends pq.Driver with driver.DriverContext compliance.
type PgSQLDriver struct {
	pq.Driver
}

// Assert interface compliance.
var (
	_ driver.Driver        = PgSQLDriver{}
	_ driver.DriverContext = PgSQLDriver{}
)

// OpenConnector implements the driver.DriverContext interface.
func (PgSQLDriver) OpenConnector(name string) (driver.Connector, error) {
	return pq.NewConnector(name)
}
