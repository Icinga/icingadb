package history

import (
	"fmt"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icingadb/pkg/icingadb/v1"
)

// UpserterEntity provides upsert for entities.
type UpserterEntity interface {
	database.Upserter
	database.Entity
}

// HistoryTableEntity is embedded by every concrete history type that has its own table.
type HistoryTableEntity struct {
	v1.EntityWithoutChecksum `json:",inline"`
}

// Upsert implements the contracts.Upserter interface.
// Update only the Id (effectively nothing).
func (hte HistoryTableEntity) Upsert() any {
	return hte
}

// HistoryEntity is embedded by every concrete history type.
type HistoryEntity struct {
	Id types.Binary `json:"event_id"`
}

// Fingerprint implements part of the contracts.Entity interface.
func (he HistoryEntity) Fingerprint() database.Fingerprinter {
	return he
}

// ID implements part of the contracts.Entity interface.
func (he HistoryEntity) ID() database.ID {
	return he.Id
}

// SetID implements part of the contracts.Entity interface.
//
// The id must be of type types.Binary. Otherwise, the method will panic.
func (he *HistoryEntity) SetID(id database.ID) {
	idBinary, ok := id.(types.Binary)
	if !ok {
		panic(fmt.Sprintf("expects types.Binary, got %T", id))
	}
	he.Id = idBinary
}

// Upsert implements the contracts.Upserter interface.
// Update only the Id (effectively nothing).
func (he HistoryEntity) Upsert() any {
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
	_ database.Entity   = (*HistoryTableEntity)(nil)
	_ database.Upserter = HistoryTableEntity{}
	_ database.Entity   = (*HistoryEntity)(nil)
	_ database.Upserter = HistoryEntity{}
	_ database.Entity   = (*HistoryMeta)(nil)
	_ database.Upserter = (*HistoryMeta)(nil)
)
