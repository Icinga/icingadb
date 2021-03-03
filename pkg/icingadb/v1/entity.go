package v1

import "github.com/icinga/icingadb/pkg/contracts"

// EntityWithoutChecksum represents entities without a checksum.
type EntityWithoutChecksum struct {
	IdMeta `json:",inline"`
}

// Fingerprint implements the contracts.Fingerprinter interface.
func (e EntityWithoutChecksum) Fingerprint() contracts.Fingerprinter {
	return e
}

// EntityWithChecksum represents entities with a checksum.
type EntityWithChecksum struct {
	EntityWithoutChecksum
	ChecksumMeta `json:",inline"`
}

// Fingerprint implements the contracts.Fingerprinter interface.
func (e EntityWithChecksum) Fingerprint() contracts.Fingerprinter {
	return e
}
