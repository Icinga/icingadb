// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package customvarflat

import (
	"fmt"
	"github.com/Icinga/icingadb/configobject"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/utils"
	"github.com/intel-go/fastjson"
	"strconv"
)

var (
	ObjectInformation configobject.ObjectInformation
	Fields            = []string{
		"id",
		"environment_id",
		"customvar_id",
		"flatname_checksum",
		"flatname",
		"flatvalue",
	}
)

type CustomvarFlat struct {
	Id           string `json:"id"`
	EnvId        string `json:"environment_id"`
	NameChecksum string `json:"name_checksum"`
	Name         string `json:"name"`
	Value        string `json:"value"`
}

func NewCustomvarFlat() connection.Row {
	c := CustomvarFlat{}

	return &c
}

func (c *CustomvarFlat) InsertValues() []interface{} {
	return nil
}

func (c *CustomvarFlat) UpdateValues() []interface{} {
	return nil
}

func (c *CustomvarFlat) GetId() string {
	return c.Id
}

func (c *CustomvarFlat) SetId(id string) {
	c.Id = id
}

func (c *CustomvarFlat) GetFinalRows() ([]connection.Row, error) {
	var values interface{} = nil
	if err := fastjson.Unmarshal([]byte(c.Value), &values); err != nil {
		return nil, err
	}

	return CollectScalarVars(c, values, c.Name, make([]string, 0)), nil
}

func CollectScalarVars(c *CustomvarFlat, value interface{}, name string, path []string) []connection.Row {
	path = append(path, name)
	switch v := value.(type) {
	case map[string]interface{}:
		var rows = []connection.Row{}
		for flatName, flatValue := range v {
			rows = append(rows, CollectScalarVars(c, flatValue, flatName, path)...)
		}

		return rows
	case []interface{}:
		var rows = []connection.Row{}
		for i, flatValue := range v {
			rows = append(rows, CollectScalarVars(c, flatValue, fmt.Sprintf("%d", i), path)...)
		}

		return rows
	default:
		flatName := ""
		for i, pathPart := range path {
			if _, err := strconv.Atoi(pathPart); err == nil {
				flatName = flatName + "[" + pathPart + "]"
			} else {
				if i > 0 {
					flatName = flatName + "."
				}
				flatName = flatName + pathPart
			}
		}

		flatValue := fmt.Sprintf("%v", v)
		return []connection.Row{
			&CustomvarFlatFinal{
				Id:               utils.Checksum(c.EnvId + flatName + flatValue),
				EnvId:            c.EnvId,
				CustomvarId:      c.Id,
				FlatNameChecksum: utils.Checksum(flatName),
				FlatName:         flatName,
				FlatValue:        flatValue,
			},
		}
	}
}

type CustomvarFlatFinal struct {
	Id               string
	EnvId            string
	CustomvarId      string
	FlatNameChecksum string
	FlatName         string
	FlatValue        string
}

func (c *CustomvarFlatFinal) InsertValues() []interface{} {
	v := c.UpdateValues()

	return append([]interface{}{utils.EncodeChecksum(c.Id)}, v...)
}

func (c *CustomvarFlatFinal) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		utils.EncodeChecksum(c.EnvId),
		utils.EncodeChecksum(c.CustomvarId),
		utils.EncodeChecksum(c.FlatNameChecksum),
		c.FlatName,
		c.FlatValue,
	)

	return v
}

func (c *CustomvarFlatFinal) GetId() string {
	return c.Id
}

func (c *CustomvarFlatFinal) SetId(id string) {
	c.Id = id
}

func (c *CustomvarFlatFinal) GetFinalRows() ([]connection.Row, error) {
	return []connection.Row{c}, nil
}

func init() {
	name := "customvar_flat"
	ObjectInformation = configobject.ObjectInformation{
		ObjectType:        name,
		RedisKey:          "customvar",
		PrimaryMySqlField: "customvar_id",
		Factory:           NewCustomvarFlat,
		HasChecksum:       false,
		BulkInsertStmt:    connection.NewBulkInsertStmt(name, Fields),
		BulkDeleteStmt:    connection.NewBulkDeleteStmt(name, "customvar_id"),
		BulkUpdateStmt:    connection.NewBulkUpdateStmt(name, Fields),
	}
}
