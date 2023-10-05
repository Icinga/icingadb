package v1

import (
	"github.com/icinga/icingadb/pkg/database"
)

// EntityWithoutChecksum represents entities without a checksum.
type EntityWithoutChecksum struct {
	IdMeta `json:",inline"`
}

// Fingerprint implements the contracts.Fingerprinter interface.
func (e EntityWithoutChecksum) Fingerprint() database.Fingerprinter {
	return e
}

// EntityWithChecksum represents entities with a checksum.
type EntityWithChecksum struct {
	EntityWithoutChecksum `json:",inline"`
	ChecksumMeta          `json:",inline"`
}

// Fingerprint implements the contracts.Fingerprinter interface.
func (e EntityWithChecksum) Fingerprint() database.Fingerprinter {
	return e
}

func NewEntityWithChecksum() database.Entity {
	return &EntityWithChecksum{}
}
