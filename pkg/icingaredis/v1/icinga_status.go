package v1

import (
	"github.com/icinga/icingadb/pkg/types"
)

// IcingaStatus defines Icinga status information.
type IcingaStatus struct {
	// Note: Icinga2Environment is not related to the environment_id used throughout Icinga DB.
	Icinga2Environment         string          `json:"environment"`
	NodeName                   string          `json:"node_name"`
	Version                    string          `json:"version"`
	ProgramStart               types.UnixMilli `json:"program_start"`
	EndpointId                 types.Binary    `json:"endpoint_id"`
	NotificationsEnabled       types.Bool      `json:"enable_notifications"`
	ActiveServiceChecksEnabled types.Bool      `json:"enable_service_checks"`
	ActiveHostChecksEnabled    types.Bool      `json:"enable_host_checks"`
	EventHandlersEnabled       types.Bool      `json:"enable_event_handlers"`
	FlapDetectionEnabled       types.Bool      `json:"enable_flapping"`
	PerformanceDataEnabled     types.Bool      `json:"enable_perfdata"`
}
