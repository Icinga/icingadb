package v1

import (
	"github.com/icinga/icingadb/pkg/types"
)

type IcingadbInstance struct {
	EntityWithoutChecksum             `json:",inline"`
	EnvironmentMeta                   `json:",inline"`
	EndpointId                        types.Binary    `json:"endpoint_id"`
	Heartbeat                         types.UnixMilli `json:"heartbeat"`
	Responsible                       types.Bool      `json:"responsible"`
	Icinga2Version                    string          `json:"icinga2_version"`
	Icinga2StartTime                  types.UnixMilli `json:"icinga2_start_Time"`
	Icinga2NotificationsEnabled       types.Bool      `json:"icinga2_notifications_enabled"`
	Icinga2ActiveServiceChecksEnabled types.Bool      `json:"icinga2_active_service_checks_enabled"`
	Icinga2ActiveHostChecksEnabled    types.Bool      `json:"icinga2_active_host_checks_enabled"`
	Icinga2EventHandlersEnabled       types.Bool      `json:"icinga2_event_handlers_enabled"`
	Icinga2FlapDetectionEnabled       types.Bool      `json:"icinga2_flap_detection_enabled"`
	Icinga2PerformanceDataEnabled     types.Bool      `json:"icinga2_performance_data_enabled"`
}
