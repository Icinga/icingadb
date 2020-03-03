// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package service

import (
	"github.com/Icinga/icingadb/configobject"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"id",
		"environment_id",
		"name_checksum",
		"properties_checksum",
		"host_id",
		"name",
		"name_ci",
		"display_name",
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

type Service struct {
	Id                    string  `json:"id"`
	EnvId                 string  `json:"environment_id"`
	NameChecksum          string  `json:"name_checksum"`
	PropertiesChecksum    string  `json:"checksum"`
	HostId                string  `json:"host_id"`
	Name                  string  `json:"name"`
	NameCi                *string `json:"name_ci"`
	DisplayName           string  `json:"display_name"`
	Checkcommand          string  `json:"checkcommand"`
	CheckcommandId        string  `json:"checkcommand_id"`
	MaxCheckAttempts      float32 `json:"max_check_attempts"`
	CheckPeriod           string  `json:"check_timeperiod"`
	CheckPeriodId         string  `json:"check_timeperiod_id"`
	CheckTimeout          float64 `json:"check_timeout"`
	CheckInterval         float64 `json:"check_interval"`
	CheckRetryInterval    float64 `json:"check_retry_interval"`
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

	return append([]interface{}{utils.EncodeChecksum(s.Id)}, v...)
}

func (s *Service) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(s.EnvId),
		utils.EncodeChecksum(s.NameChecksum),
		utils.EncodeChecksum(s.PropertiesChecksum),
		utils.EncodeChecksum(s.HostId),
		s.Name,
		s.NameCi,
		s.DisplayName,
		s.Checkcommand,
		utils.EncodeChecksum(s.CheckcommandId),
		s.MaxCheckAttempts,
		s.CheckPeriod,
		utils.EncodeChecksum(s.CheckPeriodId),
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
		utils.EncodeChecksum(s.EventcommandId),
		utils.Bool[s.IsVolatile],
		utils.EncodeChecksum(s.ActionUrlId),
		utils.EncodeChecksum(s.NotesUrlId),
		s.Notes,
		utils.EncodeChecksum(s.IconImageId),
		s.IconImageAlt,
		s.Zone,
		utils.EncodeChecksum(s.ZoneId),
		s.CommandEndpoint,
		utils.EncodeChecksum(s.CommandEndpointId),
	)

	return v
}

func (s *Service) GetId() string {
	return s.Id
}

func (s *Service) SetId(id string) {
	s.Id = id
}

func (s *Service) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{s}, nil
}

func init() {
	name := "service"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 name,
		PrimaryMySqlField:        "id",
		Factory:                  NewService,
		HasChecksum:              true,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "service",
	}
}
