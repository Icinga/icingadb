package history

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

type UpserterEntity interface {
	contracts.Upserter
	contracts.Entity
}

type HistoryTableEntity struct {
	Id types.UUID `json:"id"`
}

// Fingerprint implements part of the contracts.Entity interface.
func (hte HistoryTableEntity) Fingerprint() contracts.Fingerprinter {
	return hte
}

// ID implements part of the contracts.Entity interface.
func (hte HistoryTableEntity) ID() contracts.ID {
	return hte.Id
}

// SetID implements part of the contracts.Entity interface.
func (hte *HistoryTableEntity) SetID(id contracts.ID) {
	hte.Id = id.(types.UUID)
}

// Upsert implements the contracts.Upserter interface.
// Update only the Id (effectively nothing).
func (hte HistoryTableEntity) Upsert() interface{} {
	return hte
}

type HistoryEntity struct {
	Id types.UUID `json:"event_id"`
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
	he.Id = id.(types.UUID)
}

// Upsert implements the contracts.Upserter interface.
// Update only the Id (effectively nothing).
func (he HistoryEntity) Upsert() interface{} {
	return he
}

type HistoryTableMeta struct {
	EnvironmentId types.Binary `json:"environment_id"`
	EndpointId    types.Binary `json:"endpoint_id"`
	ObjectType    string       `json:"object_type"`
	HostId        types.Binary `json:"host_id"`
	ServiceId     types.Binary `json:"service_id"`
}

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
