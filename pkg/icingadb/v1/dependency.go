package v1

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
)

type Dependency struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	NameMeta              `json:",inline"`
	RedundancyGroupId     types.Binary `json:"redundancy_group_id"`
}

type Redundancygroup struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	Name                  string `json:"name"`
	DisplayName           string `json:"display_name"`
}

// TableName implements [database.TableNamer].
//
// Unless I am missing something, there is no way to change a type's name for the Redis, see common.SyncSubject.Name().
func (r Redundancygroup) TableName() string {
	return "redundancy_group"
}

type RedundancygroupState struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	RedundancyGroupId     types.Binary    `json:"redundancy_group_id"`
	Failed                types.Bool      `json:"failed"`
	LastStateChange       types.UnixMilli `json:"last_state_change"`
}

// TableName implements [database.TableNamer].
func (r RedundancygroupState) TableName() string {
	return "redundancy_group_state"
}

type DependencyNode struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	HostId                types.Binary `json:"host_id"`
	ServiceId             types.Binary `json:"service_id"`
	RedundancyGroupId     types.Binary `json:"redundancy_group_id"`
}

type DependencyEdge struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	FromNodeId            types.Binary `json:"from_node_id"`
	ToNodeId              types.Binary `json:"to_node_id"`
	DependencyId          types.Binary `json:"dependency_id"`
}

func NewDependency() database.Entity {
	return &Dependency{}
}

func NewRedundancygroup() database.Entity {
	return &Redundancygroup{}
}

func NewRedundancygroupState() database.Entity {
	return &RedundancygroupState{}
}

func NewDependencyNode() database.Entity {
	return &DependencyNode{}
}

func NewDependencyEdge() database.Entity {
	return &DependencyEdge{}
}
