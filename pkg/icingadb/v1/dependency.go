package v1

import (
	"bytes"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
)

type Redundancygroup struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	DisplayName           string `json:"display_name"`
}

// TableName implements [database.TableNamer].
func (r *Redundancygroup) TableName() string {
	return "redundancy_group"
}

type RedundancygroupState struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	RedundancyGroupId     types.Binary    `json:"redundancy_group_id"`
	Failed                types.Bool      `json:"failed"`
	IsReachable           types.Bool      `json:"is_reachable"`
	LastStateChange       types.UnixMilli `json:"last_state_change"`
}

// TableName implements [database.TableNamer].
func (r *RedundancygroupState) TableName() string {
	return "redundancy_group_state"
}

// Equal implements the [contracts.Equaler] interface.
func (r *RedundancygroupState) Equal(other any) bool {
	if o, ok := other.(*RedundancygroupState); ok {
		return bytes.Equal(r.Id, o.Id) &&
			bytes.Equal(r.EnvironmentId, o.EnvironmentId) &&
			bytes.Equal(r.RedundancyGroupId, o.RedundancyGroupId) &&
			r.Failed == o.Failed &&
			r.IsReachable == o.IsReachable &&
			r.LastStateChange.Time().Equal(o.LastStateChange.Time())
	}

	return false
}

type DependencyNode struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	HostId                types.Binary `json:"host_id"`
	ServiceId             types.Binary `json:"service_id"`
	RedundancyGroupId     types.Binary `json:"redundancy_group_id"`
}

type DependencyEdgeState struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	Failed                types.Bool `json:"failed"`
}

// Equal implements the [contracts.Equaler] interface.
func (es *DependencyEdgeState) Equal(other any) bool {
	if other, ok := other.(*DependencyEdgeState); ok {
		return bytes.Equal(es.Id, other.Id) &&
			bytes.Equal(es.EnvironmentId, other.EnvironmentId) &&
			es.Failed == other.Failed
	}

	return false
}

type DependencyEdge struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	FromNodeId            types.Binary `json:"from_node_id"`
	ToNodeId              types.Binary `json:"to_node_id"`
	DependencyEdgeStateId types.Binary `json:"dependency_edge_state_id"`
	DisplayName           string       `json:"display_name"`
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

func NewDependencyEdgeState() database.Entity {
	return &DependencyEdgeState{}
}

func NewDependencyEdge() database.Entity {
	return &DependencyEdge{}
}
