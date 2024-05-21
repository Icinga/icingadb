package database

import (
	"database/sql/driver"
	"github.com/jmoiron/sqlx/reflectx"
	"reflect"
	"sync"
)

// ColumnMap provides a cached mapping of structs exported fields to their database column names.
type ColumnMap interface {
	// Columns returns database column names for a struct's exported fields in a cached manner.
	// Thus, the returned slice MUST NOT be modified directly.
	// By default, all exported struct fields are mapped to database column names using snake case notation.
	// The - (hyphen) directive for the db tag can be used to exclude certain fields.
	Columns(any) []string
}

// NewColumnMap returns a new ColumnMap.
func NewColumnMap(mapper *reflectx.Mapper) ColumnMap {
	return &columnMap{
		cache:  make(map[reflect.Type][]string),
		mapper: mapper,
	}
}

type columnMap struct {
	mutex  sync.Mutex
	cache  map[reflect.Type][]string
	mapper *reflectx.Mapper
}

func (m *columnMap) Columns(subject any) []string {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	t, ok := subject.(reflect.Type)
	if !ok {
		t = reflect.TypeOf(subject)
	}

	columns, ok := m.cache[t]
	if !ok {
		columns = m.getColumns(t)
		m.cache[t] = columns
	}

	return columns
}

func (m *columnMap) getColumns(t reflect.Type) []string {
	fields := m.mapper.TypeMap(t).Names
	columns := make([]string, 0, len(fields))

FieldLoop:
	for _, f := range fields {
		// If one of the parent fields implements the driver.Valuer interface, the field can be ignored.
		for parent := f.Parent; parent != nil && parent.Zero.IsValid(); parent = parent.Parent {
			// Check for pointer types.
			if _, ok := reflect.New(parent.Field.Type).Interface().(driver.Valuer); ok {
				continue FieldLoop
			}
			// Check for non-pointer types.
			if _, ok := reflect.Zero(parent.Field.Type).Interface().(driver.Valuer); ok {
				continue FieldLoop
			}
		}

		columns = append(columns, f.Path)
	}

	// Shrink/reduce slice length and capacity:
	// For a three-index slice (slice[a:b:c]), the length of the returned slice is b-a and the capacity is c-a.
	return columns[0:len(columns):len(columns)]
}
