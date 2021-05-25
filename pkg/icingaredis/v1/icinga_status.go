package v1

import (
	"crypto/sha1"
	"github.com/icinga/icingadb/pkg/types"
)

type IcingaStatus struct {
	Environment                string          `json:"environment"`
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

func (s *IcingaStatus) EnvironmentID() types.Binary {
	chksm := sha1.Sum([]byte(s.Environment))

	return chksm[:]
}
