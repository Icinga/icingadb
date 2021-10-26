package history

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/types"
)

// UpserterEntity provides upsert for entities.
type UpserterEntity interface {
	contracts.Upserter
	contracts.Entity
}

// HistoryTableEntity is embedded by every concrete history type that has its own table.
type HistoryTableEntity struct {
	v1.EntityWithoutChecksum `json:",inline"`
}

// Upsert implements the contracts.Upserter interface.
// Update only the Id (effectively nothing).
func (hte HistoryTableEntity) Upsert() interface{} {
	return hte
}

// HistoryEntity is embedded by every concrete history type.
type HistoryEntity struct {
	Id types.Binary `json:"event_id"`
}

// Fingerprint implements part of the contracts.Entity interface.
func (he HistoryEntity) Fingerprint() contracts.Fingerprinter {
	return he
}

// ID implements part of the contracts.Entity interface.
func (he HistoryEntity) ID() contracts.ID {
	return he.Id
}

// SetID implements part of the contracts.Entity interface.
func (he *HistoryEntity) SetID(id contracts.ID) {
	he.Id = id.(types.Binary)
}

// Upsert implements the contracts.Upserter interface.
// Update only the Id (effectively nothing).
func (he HistoryEntity) Upsert() interface{} {
	return he
}

// HistoryTableMeta is embedded by every concrete history type that has its own table.
type HistoryTableMeta struct {
	EnvironmentId types.Binary `json:"environment_id"`
	EndpointId    types.Binary `json:"endpoint_id"`
	ObjectType    string       `json:"object_type"`
	HostId        types.Binary `json:"host_id"`
	ServiceId     types.Binary `json:"service_id"`
}

// HistoryMeta is embedded by every concrete history type that belongs to the history table.
type HistoryMeta struct {
	HistoryEntity `json:",inline"`
	EnvironmentId types.Binary `json:"environment_id"`
	EndpointId    types.Binary `json:"endpoint_id"`
	ObjectType    string       `json:"object_type"`
	HostId        types.Binary `json:"host_id"`
	ServiceId     types.Binary `json:"service_id"`
	EventType     string       `json:"event_type"`
}

// Assert interface compliance.
var (
	_ contracts.Entity   = (*HistoryTableEntity)(nil)
	_ contracts.Upserter = HistoryTableEntity{}
	_ contracts.Entity   = (*HistoryEntity)(nil)
	_ contracts.Upserter = HistoryEntity{}
	_ contracts.Entity   = (*HistoryMeta)(nil)
	_ contracts.Upserter = (*HistoryMeta)(nil)
)
