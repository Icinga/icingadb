package icingadb

import (
	"github.com/icinga/icingadb/pkg/database"
)

// ScopedEntity combines an entity and a scope that specifies
// the WHERE conditions that entities of the
// enclosed entity type must satisfy in order to be SELECTed.
type ScopedEntity struct {
	database.Entity
	scope interface{}
}

// Scope implements the contracts.Scoper interface.
func (e ScopedEntity) Scope() interface{} {
	return e.scope
}

// TableName implements the contracts.TableNamer interface.
func (e ScopedEntity) TableName() string {
	return database.TableName(e.Entity)
}

// NewScopedEntity returns a new ScopedEntity.
func NewScopedEntity(entity database.Entity, scope interface{}) *ScopedEntity {
	return &ScopedEntity{
		Entity: entity,
		scope:  scope,
	}
}
