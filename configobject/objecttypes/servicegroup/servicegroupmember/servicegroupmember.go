package servicegroupmember

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/utils"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"id",
		"servicegroup_id",
		"service_id",
		"environment_id",
	}
)

type ServicegroupMember struct {
	Id             string `json:"id"`
	ServicegroupId string `json:"group_id"`
	ServiceId      string `json:"object_id"`
	EnvId          string `json:"environment_id"`
}

func NewServicegroupMember() connection.Row {
	s := ServicegroupMember{}
	return &s
}

func (s *ServicegroupMember) InsertValues() []interface{} {
	v := s.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(s.Id)}, v...)
}

func (s *ServicegroupMember) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(s.ServicegroupId),
		utils.EncodeChecksum(s.ServiceId),
		utils.EncodeChecksum(s.EnvId),
	)

	return v
}

func (s *ServicegroupMember) GetId() string {
	return s.Id
}

func (s *ServicegroupMember) SetId(id string) {
	s.Id = id
}

func (s *ServicegroupMember) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{s}, nil
}

func init() {
	name := "servicegroup_member"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:               name,
		RedisKey:                 "service:groupmember",
		PrimaryMySqlField:        "id",
		Factory:                  NewServicegroupMember,
		HasChecksum:              false,
		BulkInsertStmt:           connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:           connection.NewBulkDeleteStmt(name, "id"),
		BulkUpdateStmt:           connection.NewBulkUpdateStmt(name, Fields),
		NotificationListenerType: "service",
	}
}
