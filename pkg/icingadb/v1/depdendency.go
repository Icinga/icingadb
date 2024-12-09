package v1

import (
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/types"
	icingadbTypes "github.com/icinga/icingadb/pkg/icingadb/types"
)

type Dependency struct {
	EntityWithChecksum   `json:",inline"`
	EnvironmentMeta      `json:",inline"`
	NameMeta             `json:",inline"`
	DisplayName          string                           `json:"display_name"`
	RedundancyGroupId    types.Binary                     `json:"redundancy_group_id"`
	TimeperiodId         types.Binary                     `json:"timeperiod_id"`
	DisableChecks        types.Bool                       `json:"disable_checks"`
	DisableNotifications types.Bool                       `json:"disable_notifications"`
	IgnoreSoftStates     types.Bool                       `json:"ignore_soft_states"`
	States               icingadbTypes.NotificationStates `json:"states"`
}

type DependencyState struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	DependencyId          types.Binary `json:"dependency_id"`
	Failed                types.Bool   `json:"failed"`
}

type Redundancygroup struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	Name                  string `json:"name"`
}

// TableName implements [database.TableNamer].
func (r Redundancygroup) TableName() string {
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

func NewDependencyState() database.Entity {
	return &DependencyState{}
}

func NewRedundancygroup() database.Entity {
	return &Redundancygroup{}
}

func NewRedundancyroupState() database.Entity {
	return &RedundancygroupState{}
}

func NewDependencyNode() database.Entity {
	return &DependencyNode{}
}

func NewDependencyEdge() database.Entity {
	return &DependencyEdge{}
}
