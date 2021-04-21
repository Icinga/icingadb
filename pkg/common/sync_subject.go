package common

import (
	"github.com/icinga/icingadb/pkg/contracts"
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
	_, withChecksum := e.(contracts.Checksumer)

	return &SyncSubject{
		entity:       e,
		factory:      factoryFunc,
		withChecksum: withChecksum,
	}
}

// Entity returns one value from the factory. Always returns the same entity.
func (s SyncSubject) Entity() contracts.Entity {
	return s.entity
}

// Factory returns the entity factory function.
func (s SyncSubject) Factory() contracts.EntityFactoryFunc {
	return s.factory
}

// WithChecksum returns whether entities from the factory implement contracts.Checksumer.
func (s SyncSubject) WithChecksum() bool {
	return s.withChecksum
}
