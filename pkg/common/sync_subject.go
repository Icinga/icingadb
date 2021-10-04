package common

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/utils"
)

// SyncSubject defines information about entities to be synchronized.
type SyncSubject struct {
	entity       contracts.Entity
	factory      contracts.EntityFactoryFunc
	withChecksum bool
}

// NewSyncSubject returns a new SyncSubject.
func NewSyncSubject(factoryFunc contracts.EntityFactoryFunc) *SyncSubject {
	e := factoryFunc()

	var factory contracts.EntityFactoryFunc
	if _, ok := e.(contracts.Initer); ok {
		factory = func() contracts.Entity {
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
func (s SyncSubject) Entity() contracts.Entity {
	return s.entity
}

// Factory returns the entity factory function that calls Init() on the created contracts.Entity if applicable.
func (s SyncSubject) Factory() contracts.EntityFactoryFunc {
	return s.factory
}

// Name returns the declared name of the entity.
func (s SyncSubject) Name() string {
	return utils.Name(s.entity)
}

// WithChecksum returns whether entities from the factory implement contracts.Checksumer.
func (s SyncSubject) WithChecksum() bool {
	return s.withChecksum
}
