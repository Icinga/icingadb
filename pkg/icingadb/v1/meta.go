package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

// ChecksumMeta is embedded by every type with a checksum.
type ChecksumMeta struct {
	PropertiesChecksum types.Binary `json:"checksum"`
}

// Checksum implements part of the contracts.Checksumer interface.
func (m ChecksumMeta) Checksum() contracts.Checksum {
	return m.PropertiesChecksum
}

// SetChecksum implements part of the contracts.Checksumer interface.
func (m *ChecksumMeta) SetChecksum(checksum contracts.Checksum) {
	m.PropertiesChecksum = checksum.(types.Binary)
}

// EnvironmentMeta is embedded by every type which belongs to an environment.
type EnvironmentMeta struct {
	EnvironmentId types.Binary `json:"environment_id"`
}

// IdMeta is embedded by every type Icinga DB should synchronize.
type IdMeta struct {
	Id types.Binary `json:"id"`
}

// ID implements part of the contracts.IDer interface.
func (m IdMeta) ID() contracts.ID {
	return m.Id
}

// SetID implements part of the contracts.IDer interface.
func (m *IdMeta) SetID(id contracts.ID) {
	m.Id = id.(types.Binary)
}

// NameMeta is embedded by every type with a name.
type NameMeta struct {
	Name         string       `json:"name"`
	NameChecksum types.Binary `json:"name_checksum"`
}

// NameCiMeta is embedded by every type with a case insensitive name.
type NameCiMeta struct {
	NameMeta
	NameCi *string `json:"name_ci"`
}

// Init implements the contracts.Initer interface.
func (n *NameCiMeta) Init() {
	n.NameCi = &n.Name
}

// Assert interface compliance.
var (
	_ contracts.Initer = (*NameCiMeta)(nil)
)
