package common

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/database"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/types"
)

// SyncSubject defines information about entities to be synchronized.
type SyncSubject struct {
	entity       database.Entity
	factory      database.EntityFactoryFunc
	withChecksum bool
}

// NewSyncSubject returns a new SyncSubject.
func NewSyncSubject(factoryFunc database.EntityFactoryFunc) *SyncSubject {
	e := factoryFunc()

	var factory database.EntityFactoryFunc
	if _, ok := e.(contracts.Initer); ok {
		factory = func() database.Entity {
			e := factoryFunc()
			e.(contracts.Initer).Init()

			return e
		}
	} else {
		factory = factoryFunc
	}

	_, withChecksum := e.(contracts.Checksumer)

	return &SyncSubject{
		entity:       e,
		factory:      factory,
		withChecksum: withChecksum,
	}
}

// Entity returns one value from the factory. Always returns the same entity.
func (s SyncSubject) Entity() database.Entity {
	return s.entity
}

// Factory returns the entity factory function that calls Init() on the created contracts.Entity if applicable.
func (s SyncSubject) Factory() database.EntityFactoryFunc {
	return s.factory
}

// FactoryForDelta behaves like Factory() unless s is WithChecksum().
// In the latter case it returns a factory for EntityWithChecksum instead.
// Rationale: Sync#ApplyDelta() uses its input entities which are WithChecksum() only for the delta itself
// and not for insertion into the database, so EntityWithChecksum is enough. And it consumes less memory.
func (s SyncSubject) FactoryForDelta() database.EntityFactoryFunc {
	if s.withChecksum {
		return v1.NewEntityWithChecksum
	}

	return s.factory
}

// Name returns the declared name of the entity.
func (s SyncSubject) Name() string {
	return types.Name(s.entity)
}

// WithChecksum returns whether entities from the factory implement contracts.Checksumer.
func (s SyncSubject) WithChecksum() bool {
	return s.withChecksum
}
