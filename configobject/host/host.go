package host

import (
	"encoding/json"
	"git.icinga.com/icingadb/icingadb-connection"
	"git.icinga.com/icingadb/icingadb-ha"
	"git.icinga.com/icingadb/icingadb-json-decoder"
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/supervisor"
	"git.icinga.com/icingadb/icingadb-utils"
	"git.icinga.com/icingadb/icingadb/benchmark"
	log "github.com/sirupsen/logrus"
	"sync"
)

var (
	BulkInsertStmt *icingadb_connection.BulkInsertStmt
	BulkDeleteStmt *icingadb_connection.BulkDeleteStmt
	UpdateStmt     *icingadb_connection.UpdateStmt
	Fields         = []string{
		"id",
		"env_id",
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

type Host struct {
	Id                    string  `json:"id"`
	EnvId                 string  `json:"env_id"`
	NameChecksum          string  `json:"name_checksum"`
	PropertiesChecksum    string  `json:"properties_checksum"`
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

func NewHost() configobject.Row {
	h := Host{
		EnvId:           icingadb_utils.DecodeChecksum(icingadb_utils.Sha1("default")),
		CheckPeriod:     "check_period",
		CheckPeriodId:   icingadb_utils.DecodeChecksum(icingadb_utils.Sha1("check_period")),
		Eventcommand:    "event_command",
		CommandEndpoint: "command_endpoint",
	}
	h.NameCi = &h.Name

	return &h
}

func (h *Host) InsertValues() []interface{} {
	v := h.UpdateValues()

	return append([]interface{}{icingadb_utils.Checksum(h.Id)}, v...)
}

func (h *Host) UpdateValues() []interface{} {
	v := make([]interface{}, 0)

	v = append(
		v,
		icingadb_utils.Checksum(h.EnvId),
		icingadb_utils.Checksum(h.NameChecksum),
		icingadb_utils.Checksum(h.PropertiesChecksum),
		icingadb_utils.Checksum(h.CustomvarsChecksum),
		icingadb_utils.Checksum(h.GroupsChecksum),
		h.Name,
		h.NameCi,
		h.DisplayName,
		h.Address,
		h.Address6,
		h.AddressBin,
		h.Address6Bin,
		h.Checkcommand,
		icingadb_utils.Checksum(h.CheckcommandId),
		h.MaxCheckAttempts,
		h.CheckPeriod,
		icingadb_utils.Checksum(h.CheckPeriodId),
		h.CheckTimeout,
		h.CheckInterval,
		h.CheckRetryInterval,
		icingadb_utils.Bool[h.ActiveChecksEnabled],
		icingadb_utils.Bool[h.PassiveChecksEnabled],
		icingadb_utils.Bool[h.EventHandlerEnabled],
		icingadb_utils.Bool[h.NotificationsEnabled],
		icingadb_utils.Bool[h.FlappingEnabled],
		h.FlappingThresholdLow,
		h.FlappingThresholdHigh,
		icingadb_utils.Bool[h.PerfdataEnabled],
		h.Eventcommand,
		icingadb_utils.Checksum(h.EventcommandId),
		icingadb_utils.Bool[h.IsVolatile],
		icingadb_utils.Checksum(h.ActionUrlId),
		icingadb_utils.Checksum(h.NotesUrlId),
		h.Notes,
		icingadb_utils.Checksum(h.IconImageId),
		h.IconImageAlt,
		h.Zone,
		icingadb_utils.Checksum(h.ZoneId),
		h.CommandEndpoint,
		icingadb_utils.Checksum(h.CommandEndpointId),
	)

	return v
}

func (h *Host) GetId() string {
	return h.Id
}

func (h *Host) SetId(id string) {
	h.Id = id
}

func init() {
	BulkInsertStmt = icingadb_connection.NewBulkInsertStmt("host", Fields)
	BulkDeleteStmt = icingadb_connection.NewBulkDeleteStmt("host")
	UpdateStmt = icingadb_connection.NewUpdateStmt("host", Fields[1:]) // Omit Id from fields
}

func SyncOperator(super *supervisor.Supervisor, chHA chan int) error {
	var (
		redisIds []string
		mysqlIds []string
		wg = sync.WaitGroup{}
	)

	//get ids from redis
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		res, err := super.Rdbw.HKeys("icinga:config:checksum:host").Result()
		if err != nil {
			super.ChErr <- err
			return
		}
		redisIds = res
	}()

	//get ids from mysql
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		mysqlIds, err = super.Dbw.SqlFetchIds("host")
		if err != nil {
			super.ChErr <- err
			return
		}
	}()

	wg.Wait()
	insert, update, delete := icingadb_utils.Delta(redisIds, mysqlIds)
	log.Infof("Insert: %d, Update: %d, Delete: %d", len(insert), len(update), len(delete))

	var (
		chInsert 		chan []string
		chInsertBack 	chan []configobject.Row
		chDelete 		chan []string
		chUpdateComp	chan []string
		chUpdate 		chan []string
		chUpdateBack	chan []configobject.Row
		wgInsert = &sync.WaitGroup{}
		wgDelete = &sync.WaitGroup{}
	)
	go func() {
		for msg := range chHA {
			switch msg {
			case icingadb_ha.Notify_IsNotResponsible:
				log.Info("Host: Lost responsibility")
				if chInsert != nil {
					close(chInsert)
				}
				if chInsertBack != nil {
					close(chInsertBack)
				}
				if chDelete != nil {
					close(chDelete)
				}
				if chUpdateComp != nil {
					close(chUpdateComp)
				}
				if chUpdate != nil {
					close(chUpdate)
				}
				if chUpdateBack != nil {
					close(chUpdateBack)
				}
			case icingadb_ha.Notify_IsResponsible:
				log.Info("Host: Got responsibility")

				chInsert = make(chan []string)
				chInsertBack = make(chan []configobject.Row)
				chDelete = make(chan []string)
				chUpdateComp = make(chan []string)
				chUpdate = make(chan []string)
				chUpdateBack = make(chan []configobject.Row)

				go InsertPrepWorker(super, chInsert, chInsertBack)
				go InsertExecWorker(super, chInsertBack, wgInsert)

				go DeleteExecWorker(super, chDelete, wgDelete)

				go UpdateCompWorker(super, chUpdateComp, chUpdate)
				go UpdatePrepWorker(super, chUpdate, chUpdateBack)
				go UpdateExecWorker(super, chUpdateBack)

				go func() {
					benchmarc := benchmark.NewBenchmark()
					wgInsert.Add(len(insert))
					chInsert <- insert
					//insert = nil
					wgInsert.Wait()
					benchmarc.Stop()
					log.Infof("Inserted %v hosts in %v seconds", len(insert), benchmarc.String())
				}()

				go func() {
					benchmarc := benchmark.NewBenchmark()
					wgDelete.Add(len(delete))
					chDelete <- delete
					//delete = nil
					wgDelete.Wait()
					benchmarc.Stop()
					log.Infof("Deleted %v hosts in %v seconds", len(delete), benchmarc.String())
				}()

				go func() {
					chUpdateComp <- update
					//update = nil
				}()
			}
		}
	}()

	return nil
}

func InsertPrepWorker(super *supervisor.Supervisor, chInsert <-chan []string, chInsertBack chan<- []configobject.Row) {
	defer log.Info("Host: Insert preparation routine stopped")

	prep := func(chunk *icingadb_connection.ConfigChunk) {
		pkgs := icingadb_json_decoder.JsonDecodePackages{
			ChBack: chInsertBack,
		}
		for i, key := range chunk.Keys {
			if chunk.Configs[i] == nil || chunk.Checksums[i] == nil {
				continue
			}
			pkg := icingadb_json_decoder.JsonDecodePackage{
				Id:           	key,
				ChecksumsRaw:	chunk.Checksums[i].(string),
				ConfigRaw:   	chunk.Configs[i].(string),
				Factory:		NewHost,
				ObjectType:		"host",
			}
			pkgs.Packages = append(pkgs.Packages, pkg)
		}

		super.ChDecode <- &pkgs
	}

	for keys := range chInsert {
		done := make(chan struct{})
		ch := super.Rdbw.PipeConfigChunks(done, keys, "host")
		go func() {
			for chunk := range ch {
				go prep(chunk)
			}
		}()
	}
}

func InsertExecWorker(super *supervisor.Supervisor, chInsertBack <-chan []configobject.Row, wg *sync.WaitGroup) {
	defer log.Info("Host: Insert exec routine stopped")

	for rows := range chInsertBack {
		go func(rows []configobject.Row) {
			super.ChErr <- super.Dbw.SqlBulkInsert(rows, BulkInsertStmt)
			wg.Add(-len(rows))
		}(rows)
	}
}

func DeleteExecWorker(super *supervisor.Supervisor, chDelete <-chan []string, wg *sync.WaitGroup) {
	defer log.Info("Host: Delete exec routine stopped")

	for keys := range chDelete {
		go func(keys []string) {
			super.ChErr <- super.Dbw.SqlBulkDelete(keys, BulkDeleteStmt)
			wg.Add(-len(keys))
		}(keys)
	}
}

type HostChecksums struct {
	NameChecksum          string  `json:"name_checksum"`
	PropertiesChecksum    string  `json:"properties_checksum"`
	CustomvarsChecksum    string  `json:"customvars_checksum"`
	GroupsChecksum        string  `json:"groups_checksum"`
}

func UpdateCompWorker(super *supervisor.Supervisor, chUpdate <-chan []string, chUpdateBack chan<- []string) {
	defer log.Info("Host: Update comparison routine stopped")

	prep := func(chunk *icingadb_connection.ChecksumChunk, mysqlChecksums map[string]map[string]string) {
		changed := make([]string, 0)
		for i, key := range chunk.Keys {
			if chunk.Checksums[i] == nil {
				continue
			}

			//TODO: Check if this can be done better (json in this func)
			redisChecksums := &HostChecksums{}
			err := json.Unmarshal([]byte(chunk.Checksums[i].(string)), redisChecksums)
			if err != nil {
				super.ChErr <- err
			}

			if redisChecksums.PropertiesChecksum != mysqlChecksums[key]["properties_checksum"] {
				changed = append(changed, key)
			}
		}
		chUpdateBack <- changed
	}

	for keys := range chUpdate {
		done := make(chan struct{})
		ch := super.Rdbw.PipeChecksumChunks(done, keys, "host")
		checksums, err := super.Dbw.SqlFetchChecksums("host", keys)
		if err != nil {
			super.ChErr <- err
		}

		go func() {
			for chunk := range ch {
				go prep(chunk, checksums)
			}
		}()
	}
}

func UpdatePrepWorker(super *supervisor.Supervisor, chUpdate <-chan []string, chUpdateBack chan<- []configobject.Row) {
	defer log.Info("Host: Update preparation routine stopped")

	prep := func(chunk *icingadb_connection.ConfigChunk) {
		pkgs := icingadb_json_decoder.JsonDecodePackages{
			ChBack: chUpdateBack,
		}
		for i, key := range chunk.Keys {
			if chunk.Configs[i] == nil || chunk.Checksums[i] == nil {
				continue
			}
			pkg := icingadb_json_decoder.JsonDecodePackage{
				Id:           	key,
				ChecksumsRaw:	chunk.Checksums[i].(string),
				ConfigRaw:   	chunk.Configs[i].(string),
				Factory:		NewHost,
				ObjectType:		"host",
			}
			pkgs.Packages = append(pkgs.Packages, pkg)
		}

		super.ChDecode <- &pkgs
	}

	for keys := range chUpdate {
		done := make(chan struct{})
		ch := super.Rdbw.PipeConfigChunks(done, keys, "host")
		go func() {
			for chunk := range ch {
				go prep(chunk)
			}
		}()
	}
}

func UpdateExecWorker(super *supervisor.Supervisor, chUpdateBack <-chan []configobject.Row) {
	defer log.Info("Host: Insert exec routine stopped")

	for rows := range chUpdateBack {
		go func(rows []configobject.Row) {
			super.ChErr <- super.Dbw.SqlBulkUpdate(rows, UpdateStmt)
			log.Infof("Updated %v hosts", len(rows))
		}(rows)
	}
}