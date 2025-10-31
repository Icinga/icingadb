package v1

import (
	"fmt"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icingadb/pkg/contracts"
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
//
// The checksum must be of type types.Binary. Otherwise, the method will panic.
func (m *ChecksumMeta) SetChecksum(checksum contracts.Checksum) {
	checksumBinary, ok := checksum.(types.Binary)
	if !ok {
		panic(fmt.Sprintf("expects types.Binary, got %T", checksum))
	}
	m.PropertiesChecksum = checksumBinary
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
func (m IdMeta) ID() database.ID {
	return m.Id
}

// SetID implements part of the contracts.IDer interface.
//
// The id must be of type types.Binary. Otherwise, the method will panic.
func (m *IdMeta) SetID(id database.ID) {
	idBinary, ok := id.(types.Binary)
	if !ok {
		panic(fmt.Sprintf("expects types.Binary, got %T", id))
	}
	m.Id = idBinary
}

// NameMeta is embedded by every type with a name.
type NameMeta struct {
	Name         string       `json:"name"`
	NameChecksum types.Binary `json:"name_checksum"`
}

// NameCiMeta is embedded by every type with a case insensitive name.
type NameCiMeta struct {
	NameMeta `json:",inline"`
	NameCi   *string `json:"-"`
}

// Init implements the contracts.Initer interface.
func (n *NameCiMeta) Init() {
	n.NameCi = &n.Name
}

// CustomvarMeta is embedded by every type with custom variables.
type CustomvarMeta struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	CustomvarId           types.Binary `json:"customvar_id"`
}

// GroupMeta is embedded by every type that represents a specific group.
type GroupMeta struct {
	EntityWithChecksum `json:",inline"`
	EnvironmentMeta    `json:",inline"`
	NameCiMeta         `json:",inline"`
	DisplayName        string       `json:"display_name"`
	ZoneId             types.Binary `json:"zone_id"`
}

// MemberMeta is embedded by every type that represents members of a specific group.
type MemberMeta struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
}

// Assert interface compliance.
var (
	_ contracts.Initer = (*NameCiMeta)(nil)
	_ contracts.Initer = (*GroupMeta)(nil)
)
