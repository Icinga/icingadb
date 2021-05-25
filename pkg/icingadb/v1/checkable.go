package v1

import (
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/types"
)

type Checkable struct {
	EntityWithChecksum    `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	NameCiMeta            `json:",inline"`
	ActionUrlId           types.Binary `json:"action_url_id"`
	ActiveChecksEnabled   types.Bool   `json:"active_checks_enabled"`
	CheckInterval         float64      `json:"check_interval"`
	CheckTimeperiod       string       `json:"check_timeperiod"`
	CheckTimeperiodId     types.Binary `json:"check_timeperiod_id"`
	CheckRetryInterval    float64      `json:"check_retry_interval"`
	CheckTimeout          float64      `json:"check_timeout"`
	Checkcommand          string       `json:"checkcommand"`
	CheckcommandId        types.Binary `json:"checkcommand_id"`
	CommandEndpoint       string       `json:"command_endpoint"`
	CommandEndpointId     types.Binary `json:"command_endpoint_id"`
	DisplayName           string       `json:"display_name"`
	EventHandlerEnabled   types.Bool   `json:"event_handler_enabled"`
	Eventcommand          string       `json:"eventcommand"`
	EventcommandId        types.Binary `json:"eventcommand_id"`
	FlappingEnabled       types.Bool   `json:"flapping_enabled"`
	FlappingThresholdHigh float64      `json:"flapping_threshold_high"`
	FlappingThresholdLow  float64      `json:"flapping_threshold_low"`
	IconImageAlt          string       `json:"icon_image_alt"`
	IconImageId           types.Binary `json:"icon_image_id"`
	IsVolatile            types.Bool   `json:"is_volatile"`
	MaxCheckAttempts      float64      `json:"max_check_attempts"`
	Notes                 string       `json:"notes"`
	NotesUrlId            types.Binary `json:"notes_url_id"`
	NotificationsEnabled  types.Bool   `json:"notifications_enabled"`
	PassiveChecksEnabled  types.Bool   `json:"passive_checks_enabled"`
	PerfdataEnabled       types.Bool   `json:"perfdata_enabled"`
	Zone                  string       `json:"zone"`
	ZoneId                types.Binary `json:"zone_id"`
}

// Assert interface compliance.
var (
	_ contracts.Initer = (*Checkable)(nil)
)
