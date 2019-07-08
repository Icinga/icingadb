package service

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields         = []string{
		"id",
		"env_id",
		"name_checksum",
		"properties_checksum",
		"customvars_checksum",
		"groups_checksum",
		"host_id",
		"name",
		"name_ci",
		"display_name",
		"checkcommand",
		"checkcommand_id",
		"max_check_attempts",
		"check_period",
		"check_period_id",
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

type Service struct {
	Id                    string  `json:"id"`
	EnvId                 string  `json:"env_id"`
	NameChecksum          string  `json:"name_checksum"`
	PropertiesChecksum    string  `json:"checksum"`
	CustomvarsChecksum    string  `json:"customvars_checksum"`
	GroupsChecksum        string  `json:"groups_checksum"`
	HostId                string  `json:"host_id"`
	Name                  string  `json:"name"`
	NameCi                *string `json:"name_ci"`
	DisplayName           string  `json:"display_name"`
	Checkcommand          string  `json:"checkcommand"`
	CheckcommandId        string  `json:"checkcommand_id"`
	MaxCheckAttempts      float32 `json:"max_check_attempts"`
	CheckPeriod           string  `json:"check_period"`
	CheckPeriodId         string  `json:"check_period_id"`
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

func NewService() connection.Row {
	s := Service{}
	s.NameCi = &s.Name

	return &s
}

func (s *Service) InsertValues() []interface{} {
	v := s.UpdateValues()

	return append([]interface{}{utils.Checksum(s.Id)}, v...)
}

func (s *Service) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.Checksum(s.EnvId),
		utils.Checksum(s.NameChecksum),
		utils.Checksum(s.PropertiesChecksum),
		utils.Checksum(s.CustomvarsChecksum),
		utils.Checksum(s.GroupsChecksum),
		utils.Checksum(s.HostId),
		s.Name,
		s.NameCi,
		s.DisplayName,
		s.Checkcommand,
		utils.Checksum(s.CheckcommandId),
		s.MaxCheckAttempts,
		s.CheckPeriod,
		utils.Checksum(s.CheckPeriodId),
		s.CheckTimeout,
		s.CheckInterval,
		s.CheckRetryInterval,
		utils.Bool[s.ActiveChecksEnabled],
		utils.Bool[s.PassiveChecksEnabled],
		utils.Bool[s.EventHandlerEnabled],
		utils.Bool[s.NotificationsEnabled],
		utils.Bool[s.FlappingEnabled],
		s.FlappingThresholdLow,
		s.FlappingThresholdHigh,
		utils.Bool[s.PerfdataEnabled],
		s.Eventcommand,
		utils.Checksum(s.EventcommandId),
		utils.Bool[s.IsVolatile],
		utils.Checksum(s.ActionUrlId),
		utils.Checksum(s.NotesUrlId),
		s.Notes,
		utils.Checksum(s.IconImageId),
		s.IconImageAlt,
		s.Zone,
		utils.Checksum(s.ZoneId),
		s.CommandEndpoint,
		utils.Checksum(s.CommandEndpointId),
	)

	return v
}

func (s *Service) GetId() string {
	return s.Id
}

func (s *Service) SetId(id string) {
	s.Id = id
}

func init() {
	ObjectInformation = configobject.ObjectInformation{
		ObjectType: "service",
		RedisKey: "service",
		Factory: NewService,
		BulkInsertStmt: connection.NewBulkInsertStmt("service", Fields),
		BulkDeleteStmt: connection.NewBulkDeleteStmt("service"),
		BulkUpdateStmt: connection.NewBulkUpdateStmt("service", Fields),
	}
}