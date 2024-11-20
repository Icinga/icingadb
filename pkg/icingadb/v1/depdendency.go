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

type RedundancyGroup struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	NameMeta              `json:",inline"`
	DisplayName           string `json:"display_name"`
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

func NewRedundancyGroup() database.Entity {
	return &RedundancyGroup{}
}

func NewDependencyNode() database.Entity {
	return &DependencyNode{}
}

func NewDependencyEdge() database.Entity {
	return &DependencyEdge{}
}
