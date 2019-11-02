package host

import (
	"github.com/icinga/icingadb/configobject"
	"github.com/icinga/icingadb/connection"
	"github.com/icinga/icingadb/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"id",
		"environment_id",
		"name_checksum",
		"properties_checksum",
		"customvars_checksum",
		"groups_checksum",
		"name",
		"name_ci",
		"display_name",
		"address",
		"address6",
		"address_bin",
		"address6_bin",
		"checkcommand",
		"checkcommand_id",
		"max_check_attempts",
		"check_timeperiod",
		"check_timeperiod_id",
		"check_timeout",
		"check_interval",
		"check_retry_interval",
		"active_checks_enabled",
		"passive_checks_enabled",
		"event_handler_enabled",
		"notifications_enabled",
		"flapping_enabled",
		"flapping_threshold_low",
		"flapping_threshold_high",
		"perfdata_enabled",
		"eventcommand",
		"eventcommand_id",
		"is_volatile",
		"action_url_id",
		"notes_url_id",
		"notes",
		"icon_image_id",
		"icon_image_alt",
		"zone",
		"zone_id",
		"command_endpoint",
		"command_endpoint_id",
	}
)

type Host struct {
	Id                    string  `json:"id"`
	EnvId                 string  `json:"environment_id"`
	NameChecksum          string  `json:"name_checksum"`
	PropertiesChecksum    string  `json:"checksum"`
	CustomvarsChecksum    string  `json:"customvars_checksum"`
	GroupsChecksum        string  `json:"groups_checksum"`
	Name                  string  `json:"name"`
	NameCi                *string `json:"name_ci"`
	DisplayName           string  `json:"display_name"`
	Address               string  `json:"address"`
	Address6              string  `json:"address6"`
	AddressBin            string  `json:"address_bin"`
	Address6Bin           string  `json:"address6_bin"`
	Checkcommand          string  `json:"checkcommand"`
	CheckcommandId        string  `json:"checkcommand_id"`
	MaxCheckAttempts      float32 `json:"max_check_attempts"`
	CheckPeriod           string  `json:"check_timeperiod"`
	CheckPeriodId         string  `json:"check_timeperiod_id"`
	CheckTimeout          float32 `json:"check_timeout"`
	CheckInterval         float32 `json:"check_interval"`
	CheckRetryInterval    float32 `json:"check_retry_interval"`
	ActiveChecksEnabled   bool    `json:"active_checks_enabled"`
	PassiveChecksEnabled  bool    `json:"passive_checks_enabled"`
	EventHandlerEnabled   bool    `json:"event_handler_enabled"`
	NotificationsEnabled  bool    `json:"notifications_enabled"`
	FlappingEnabled       bool    `json:"flapping_enabled"`
	FlappingThresholdLow  float32 `json:"flapping_threshold_low"`
	FlappingThresholdHigh float32 `json:"flapping_threshold_high"`
	PerfdataEnabled       bool    `json:"perfdata_enabled"`
	Eventcommand          string  `json:"eventcommand"`
	EventcommandId        string  `json:"eventcommand_id"`
	IsVolatile            bool    `json:"is_volatile"`
	ActionUrlId           string  `json:"action_url_id"`
	NotesUrlId            string  `json:"notes_url_id"`
	Notes                 string  `json:"notes"`
	IconImageId           string  `json:"icon_image_id"`
	IconImageAlt          string  `json:"icon_image_alt"`
	Zone                  string  `json:"zone"`
	ZoneId                string  `json:"zone_id"`
	CommandEndpoint       string  `json:"command_endpoint"`
	CommandEndpointId     string  `json:"command_endpoint_id"`
}

func NewHost() connection.Row {
	h := Host{}
	h.NameCi = &h.Name

	return &h
}

func (h *Host) InsertValues() []interface{} {
	v := h.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(h.Id)}, v...)
}

func (h *Host) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(h.EnvId),
		utils.EncodeChecksum(h.NameChecksum),
		utils.EncodeChecksum(h.PropertiesChecksum),
		utils.EncodeChecksum(h.CustomvarsChecksum),
		utils.EncodeChecksum(h.GroupsChecksum),
		h.Name,
		h.NameCi,
		h.DisplayName,
		h.Address,
		h.Address6,
		h.AddressBin,
		h.Address6Bin,
		h.Checkcommand,
		utils.EncodeChecksum(h.CheckcommandId),
		h.MaxCheckAttempts,
		h.CheckPeriod,
		utils.EncodeChecksum(h.CheckPeriodId),
		h.CheckTimeout,
		h.CheckInterval,
		h.CheckRetryInterval,
		utils.Bool[h.ActiveChecksEnabled],
		utils.Bool[h.PassiveChecksEnabled],
		utils.Bool[h.EventHandlerEnabled],
		utils.Bool[h.NotificationsEnabled],
		utils.Bool[h.FlappingEnabled],
		h.FlappingThresholdLow,
		h.FlappingThresholdHigh,
		utils.Bool[h.PerfdataEnabled],
		h.Eventcommand,
		utils.EncodeChecksum(h.EventcommandId),
		utils.Bool[h.IsVolatile],
		utils.EncodeChecksum(h.ActionUrlId),
		utils.EncodeChecksum(h.NotesUrlId),
		h.Notes,
		utils.EncodeChecksum(h.IconImageId),
		h.IconImageAlt,
		h.Zone,
		utils.EncodeChecksum(h.ZoneId),
		h.CommandEndpoint,
		utils.EncodeChecksum(h.CommandEndpointId),
	)

	return v
}

func (h *Host) GetId() string {
	return h.Id
}

func (h *Host) SetId(id string) {
	h.Id = id
}

func (h *Host) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{h}, nil
}

func init() {
	name := "host"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 name,
		PrimaryMySqlField:        "id",
		Factory:                  NewHost,
		HasChecksum:              true,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "host",
	}
}
