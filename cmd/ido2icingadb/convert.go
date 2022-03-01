package main

import (
	"database/sql"
	"github.com/icinga/icingadb/pkg/contracts"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingadb/v1/history"
	icingadbTypes "github.com/icinga/icingadb/pkg/types"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"strconv"
	"strings"
	"time"
)

const acknowledgementMigrationQuery = "SELECT ah.acknowledgement_id, UNIX_TIMESTAMP(ah.entry_time) entry_time, " +
	"ah.entry_time_usec, ah.acknowledgement_type, ah.author_name, ah.comment_data, ah.is_sticky, " +
	"ah.persistent_comment, UNIX_TIMESTAMP(ah.end_time) end_time, o.objecttype_id, o.name1, " +
	"IFNULL(o.name2, '') name2 " +
	"FROM icinga_acknowledgements ah USE INDEX (PRIMARY) " +
	"INNER JOIN icinga_objects o ON o.object_id=ah.object_id " +
	"WHERE ah.acknowledgement_id > :checkpoint " + // where we were interrupted
	"ORDER BY ah.acknowledgement_id " + // allows computeProgress() not to check all IDO rows for whether migrated
	"LIMIT :bulk"

// AckClear updates an already migrated ack event with the clear event info.
type AckClear struct {
	Id        icingadbTypes.Binary
	ClearTime icingadbTypes.UnixMilli
}

// Assert interface compliance.
var _ contracts.TableNamer = (*AckClear)(nil)

// TableName implements the contracts.TableNamer interface.
func (*AckClear) TableName() string {
	return "acknowledgement_history"
}

type acknowledgementRow = struct {
	AcknowledgementId   uint64
	EntryTime           int64
	EntryTimeUsec       uint32
	AcknowledgementType uint8
	AuthorName          sql.NullString
	CommentData         sql.NullString
	IsSticky            uint8
	PersistentComment   uint8
	EndTime             sql.NullInt64
	ObjecttypeId        uint8
	Name1               string
	Name2               string
}

func convertAcknowledgementRows(
	env string, envId, endpointId icingadbTypes.Binary,
	selectCache func(dest interface{}, query string, args ...interface{}), _ *sqlx.Tx, ir interface{},
) (icingaDbUpdates, icingaDbInserts [][]interface{}, checkpoint interface{}) {
	idoRows := ir.([]acknowledgementRow)
	if len(idoRows) < 1 {
		return
	}

	var cached []struct {
		HistoryId     uint64
		EventTime     int64
		EventTimeUsec uint32
	}
	selectCache(
		&cached, "SELECT history_id, event_time, event_time_usec FROM end_start_time WHERE history_id BETWEEN ? AND ?",
		idoRows[0].AcknowledgementId, idoRows[len(idoRows)-1].AcknowledgementId,
	)

	// Needed for set time (see below).
	cachedById := make(map[uint64]icingadbTypes.UnixMilli, len(cached))
	for _, c := range cached {
		cachedById[c.HistoryId] = convertTime(c.EventTime, c.EventTimeUsec)
	}

	var acknowledgementHistory, acknowledgementHistoryUpdates, allHistory []interface{}
	for _, row := range idoRows {
		ts := convertTime(row.EntryTime, row.EntryTimeUsec)

		// Needed for ID (see below).
		var set icingadbTypes.UnixMilli
		if row.AcknowledgementType == 0 { // clear
			var ok bool
			set, ok = cachedById[row.AcknowledgementId]

			if !ok {
				continue
			}
		} else {
			set = ts
		}

		name := row.Name1
		if row.Name2 != "" {
			name += "!" + row.Name2
		}

		typ := objectTypes[row.ObjecttypeId]
		hostId := calcObjectId(env, row.Name1)
		serviceId := calcServiceId(env, row.Name1, row.Name2)
		setTime := float64(utils.UnixMilli(set.Time()))
		acknowledgementHistoryId := hashAny([]interface{}{env, name, setTime})

		if row.AcknowledgementType == 0 { // clear
			// The set counterpart should already have been inserted.
			acknowledgementHistoryUpdates = append(acknowledgementHistoryUpdates, &AckClear{
				acknowledgementHistoryId, ts,
			})

			h := &history.HistoryAck{
				HistoryMeta: history.HistoryMeta{
					HistoryEntity: history.HistoryEntity{
						Id: hashAny([]interface{}{env, "ack_clear", name, setTime}),
					},
					EnvironmentId: envId,
					EndpointId:    endpointId,
					ObjectType:    typ,
					HostId:        hostId,
					ServiceId:     serviceId,
					EventType:     "ack_clear",
				},
				AcknowledgementHistoryId: acknowledgementHistoryId,
				SetTime:                  set,
				ClearTime:                ts,
			}

			h.EventTime.History = h
			allHistory = append(allHistory, h)
		} else { // set
			acknowledgementHistory = append(acknowledgementHistory, &history.AcknowledgementHistory{
				EntityWithoutChecksum: v1.EntityWithoutChecksum{
					IdMeta: v1.IdMeta{Id: acknowledgementHistoryId},
				},
				HistoryTableMeta: history.HistoryTableMeta{
					EnvironmentId: envId,
					EndpointId:    endpointId,
					ObjectType:    typ,
					HostId:        hostId,
					ServiceId:     serviceId,
				},
				SetTime:      set,
				Author:       icingadbTypes.String{NullString: row.AuthorName},
				Comment:      icingadbTypes.String{NullString: row.CommentData},
				ExpireTime:   convertTime(row.EndTime.Int64, 0),
				IsPersistent: icingadbTypes.Bool{Bool: row.PersistentComment != 0, Valid: true},
				IsSticky:     icingadbTypes.Bool{Bool: row.IsSticky != 0, Valid: true},
			})

			h := &history.HistoryAck{
				HistoryMeta: history.HistoryMeta{
					HistoryEntity: history.HistoryEntity{
						Id: hashAny([]interface{}{env, "ack_set", name, setTime}),
					},
					EnvironmentId: envId,
					EndpointId:    endpointId,
					ObjectType:    typ,
					HostId:        hostId,
					ServiceId:     serviceId,
					EventType:     "ack_set",
				},
				AcknowledgementHistoryId: acknowledgementHistoryId,
				SetTime:                  set,
			}

			h.EventTime.History = h
			allHistory = append(allHistory, h)
		}

		checkpoint = row.AcknowledgementId
	}

	icingaDbUpdates = [][]interface{}{acknowledgementHistoryUpdates}
	icingaDbInserts = [][]interface{}{acknowledgementHistory, allHistory}
	return
}

const commentMigrationQuery = "SELECT ch.commenthistory_id, UNIX_TIMESTAMP(ch.entry_time) entry_time, " +
	"ch.entry_time_usec, ch.entry_type, ch.author_name, ch.comment_data, ch.is_persistent, " +
	"IFNULL(UNIX_TIMESTAMP(ch.expiration_time), 0) expiration_time, " +
	"IFNULL(UNIX_TIMESTAMP(ch.deletion_time), 0) deletion_time, ch.deletion_time_usec, ch.name, " +
	"o.objecttype_id, o.name1, IFNULL(o.name2, '') name2 " +
	"FROM icinga_commenthistory ch USE INDEX (PRIMARY) " +
	"INNER JOIN icinga_objects o ON o.object_id=ch.object_id " +
	"WHERE ch.commenthistory_id > :checkpoint " + // where we were interrupted
	"ORDER BY ch.commenthistory_id " + // allows computeProgress() not to check all IDO rows for whether migrated
	"LIMIT :bulk"

type commentRow = struct {
	CommenthistoryId uint64
	EntryTime        int64
	EntryTimeUsec    uint32
	EntryType        uint8
	AuthorName       string
	CommentData      string
	IsPersistent     uint8
	ExpirationTime   int64
	DeletionTime     int64
	DeletionTimeUsec uint32
	Name             string
	ObjecttypeId     uint8
	Name1            string
	Name2            string
}

func convertCommentRows(
	env string, envId, endpointId icingadbTypes.Binary,
	_ func(interface{}, string, ...interface{}), _ *sqlx.Tx, ir interface{},
) (_, icingaDbInserts [][]interface{}, checkpoint interface{}) {
	idoRows := ir.([]commentRow)
	var commentHistory, allHistory []interface{}

	for _, row := range idoRows {
		id := calcObjectId(env, row.Name)
		typ := objectTypes[row.ObjecttypeId]
		hostId := calcObjectId(env, row.Name1)
		serviceId := calcServiceId(env, row.Name1, row.Name2)
		entryTime := convertTime(row.EntryTime, row.EntryTimeUsec)
		removeTime := convertTime(row.DeletionTime, row.DeletionTimeUsec)
		expireTime := convertTime(row.ExpirationTime, 0)

		commentHistory = append(commentHistory, &history.CommentHistory{
			CommentHistoryEntity: history.CommentHistoryEntity{CommentId: id},
			HistoryTableMeta: history.HistoryTableMeta{
				EnvironmentId: envId,
				EndpointId:    endpointId,
				ObjectType:    typ,
				HostId:        hostId,
				ServiceId:     serviceId,
			},
			CommentHistoryUpserter: history.CommentHistoryUpserter{
				RemoveTime:     removeTime,
				HasBeenRemoved: icingadbTypes.Bool{Bool: !removeTime.Time().IsZero(), Valid: true},
			},
			EntryTime:    entryTime,
			Author:       row.AuthorName,
			Comment:      row.CommentData,
			EntryType:    icingadbTypes.CommentType(row.EntryType),
			IsPersistent: icingadbTypes.Bool{Bool: row.IsPersistent != 0, Valid: true},
			IsSticky:     icingadbTypes.Bool{Bool: false, Valid: true},
			ExpireTime:   expireTime,
		})

		h1 := &history.HistoryComment{
			HistoryMeta: history.HistoryMeta{
				HistoryEntity: history.HistoryEntity{Id: hashAny([]string{env, "comment_add", row.Name})},
				EnvironmentId: envId,
				EndpointId:    endpointId,
				ObjectType:    typ,
				HostId:        hostId,
				ServiceId:     serviceId,
				EventType:     "comment_add",
			},
			CommentHistoryId: id,
			EntryTime:        entryTime,
		}

		h1.EventTime.History = h1
		allHistory = append(allHistory, h1)

		if !removeTime.Time().IsZero() { // remove
			h2 := &history.HistoryComment{
				HistoryMeta: history.HistoryMeta{
					HistoryEntity: history.HistoryEntity{Id: hashAny([]string{env, "comment_remove", row.Name})},
					EnvironmentId: envId,
					EndpointId:    endpointId,
					ObjectType:    typ,
					HostId:        hostId,
					ServiceId:     serviceId,
					EventType:     "comment_remove",
				},
				CommentHistoryId: id,
				EntryTime:        entryTime,
				RemoveTime:       removeTime,
				ExpireTime:       expireTime,
			}

			h2.EventTime.History = h2
			allHistory = append(allHistory, h2)
		}

		checkpoint = row.CommenthistoryId
	}

	icingaDbInserts = [][]interface{}{commentHistory, allHistory}
	return
}

const downtimeMigrationQuery = "SELECT dh.downtimehistory_id, UNIX_TIMESTAMP(dh.entry_time) entry_time, " +
	"dh.author_name, dh.comment_data, dh.is_fixed, dh.duration, " +
	"UNIX_TIMESTAMP(dh.scheduled_start_time) scheduled_start_time, " +
	"IFNULL(UNIX_TIMESTAMP(dh.scheduled_end_time), 0) scheduled_end_time, " +
	"IFNULL(UNIX_TIMESTAMP(dh.actual_start_time), 0) actual_start_time, dh.actual_start_time_usec, " +
	"IFNULL(UNIX_TIMESTAMP(dh.actual_end_time), 0) actual_end_time, dh.actual_end_time_usec, dh.was_cancelled, " +
	"IFNULL(UNIX_TIMESTAMP(dh.trigger_time), 0) trigger_time, dh.name, o.objecttype_id, o.name1, " +
	"IFNULL(o.name2, '') name2, IFNULL(sd.name, '') triggered_by " +
	"FROM icinga_downtimehistory dh USE INDEX (PRIMARY) " +
	"INNER JOIN icinga_objects o ON o.object_id=dh.object_id " +
	"LEFT JOIN icinga_scheduleddowntime sd ON sd.scheduleddowntime_id=dh.triggered_by_id " +
	"WHERE dh.downtimehistory_id > :checkpoint " + // where we were interrupted
	"ORDER BY dh.downtimehistory_id " + // allows computeProgress() not to check all IDO rows for whether migrated
	"LIMIT :bulk"

type downtimeRow = struct {
	DowntimehistoryId   uint64
	EntryTime           int64
	AuthorName          string
	CommentData         string
	IsFixed             uint8
	Duration            int64
	ScheduledStartTime  int64
	ScheduledEndTime    int64
	ActualStartTime     int64
	ActualStartTimeUsec uint32
	ActualEndTime       int64
	ActualEndTimeUsec   uint32
	WasCancelled        uint8
	TriggerTime         int64
	Name                string
	ObjecttypeId        uint8
	Name1               string
	Name2               string
	TriggeredBy         string
}

func convertDowntimeRows(
	env string, envId, endpointId icingadbTypes.Binary,
	_ func(interface{}, string, ...interface{}), _ *sqlx.Tx, ir interface{},
) (_, icingaDbInserts [][]interface{}, checkpoint interface{}) {
	idoRows := ir.([]downtimeRow)
	var downtimeHistory, allHistory []interface{}

	for _, row := range idoRows {
		id := calcObjectId(env, row.Name)
		typ := objectTypes[row.ObjecttypeId]
		hostId := calcObjectId(env, row.Name1)
		serviceId := calcServiceId(env, row.Name1, row.Name2)
		scheduledStart := convertTime(row.ScheduledStartTime, 0)
		scheduledEnd := convertTime(row.ScheduledEndTime, 0)
		triggerTime := convertTime(row.TriggerTime, 0)
		actualStart := convertTime(row.ActualStartTime, row.ActualStartTimeUsec)
		actualEnd := convertTime(row.ActualEndTime, row.ActualEndTimeUsec)
		var startTime, endTime, cancelTime icingadbTypes.UnixMilli

		if scheduledEnd.Time().IsZero() {
			scheduledEnd = icingadbTypes.UnixMilli(scheduledStart.Time().Add(time.Duration(row.Duration) * time.Second))
		}

		if actualStart.Time().IsZero() {
			startTime = scheduledStart
		} else {
			startTime = actualStart
		}

		if actualEnd.Time().IsZero() {
			endTime = scheduledEnd
		} else {
			endTime = actualEnd
		}

		if triggerTime.Time().IsZero() {
			triggerTime = startTime
		}

		if row.WasCancelled != 0 {
			cancelTime = actualEnd
		}

		downtimeHistory = append(downtimeHistory, &history.DowntimeHistory{
			DowntimeHistoryEntity: history.DowntimeHistoryEntity{DowntimeId: id},
			HistoryTableMeta: history.HistoryTableMeta{
				EnvironmentId: envId,
				EndpointId:    endpointId,
				ObjectType:    typ,
				HostId:        hostId,
				ServiceId:     serviceId,
			},
			DowntimeHistoryUpserter: history.DowntimeHistoryUpserter{
				HasBeenCancelled: icingadbTypes.Bool{Bool: row.WasCancelled != 0, Valid: true},
				CancelTime:       cancelTime,
			},
			TriggeredById:      calcObjectId(env, row.TriggeredBy),
			EntryTime:          convertTime(row.EntryTime, 0),
			Author:             row.AuthorName,
			Comment:            row.CommentData,
			IsFlexible:         icingadbTypes.Bool{Bool: row.IsFixed == 0, Valid: true},
			FlexibleDuration:   uint64(row.Duration) * 1000,
			ScheduledStartTime: scheduledStart,
			ScheduledEndTime:   scheduledEnd,
			StartTime:          startTime,
			EndTime:            endTime,
			TriggerTime:        triggerTime,
		})

		h1 := &history.HistoryDowntime{
			HistoryMeta: history.HistoryMeta{
				HistoryEntity: history.HistoryEntity{Id: hashAny([]string{env, "downtime_start", row.Name})},
				EnvironmentId: envId,
				EndpointId:    endpointId,
				ObjectType:    typ,
				HostId:        hostId,
				ServiceId:     serviceId,
				EventType:     "downtime_start",
			},
			DowntimeHistoryId: id,
			StartTime:         startTime,
		}

		h1.EventTime.History = h1
		allHistory = append(allHistory, h1)

		if !actualEnd.Time().IsZero() { // remove
			h2 := &history.HistoryDowntime{
				HistoryMeta: history.HistoryMeta{
					HistoryEntity: history.HistoryEntity{Id: hashAny([]string{env, "downtime_end", row.Name})},
					EnvironmentId: envId,
					EndpointId:    endpointId,
					ObjectType:    typ,
					HostId:        hostId,
					ServiceId:     serviceId,
					EventType:     "downtime_end",
				},
				DowntimeHistoryId: id,
				StartTime:         startTime,
				CancelTime:        cancelTime,
				EndTime:           endTime,
				HasBeenCancelled:  icingadbTypes.Bool{Bool: row.WasCancelled != 0, Valid: true},
			}

			h2.EventTime.History = h2
			allHistory = append(allHistory, h2)
		}

		checkpoint = row.DowntimehistoryId
	}

	icingaDbInserts = [][]interface{}{downtimeHistory, allHistory}
	return
}

const flappingMigrationQuery = "SELECT fh.flappinghistory_id, UNIX_TIMESTAMP(fh.event_time) event_time, " +
	"fh.event_time_usec, fh.event_type, fh.percent_state_change, fh.low_threshold, " +
	"fh.high_threshold, o.objecttype_id, o.name1, IFNULL(o.name2, '') name2 " +
	"FROM icinga_flappinghistory fh USE INDEX (PRIMARY) " +
	"INNER JOIN icinga_objects o ON o.object_id=fh.object_id " +
	"WHERE fh.flappinghistory_id > :checkpoint " + // where we were interrupted
	"ORDER BY fh.flappinghistory_id " + // allows computeProgress() not to check all IDO rows for whether migrated
	"LIMIT :bulk"

// FlappingEnd updates an already migrated start event with the end event info.
type FlappingEnd struct {
	Id      icingadbTypes.Binary
	EndTime icingadbTypes.UnixMilli
}

// Assert interface compliance.
var _ contracts.TableNamer = (*FlappingEnd)(nil)

// TableName implements the contracts.TableNamer interface.
func (*FlappingEnd) TableName() string {
	return "flapping_history"
}

type flappingRow = struct {
	FlappinghistoryId  uint64
	EventTime          int64
	EventTimeUsec      uint32
	EventType          uint16
	PercentStateChange sql.NullFloat64
	LowThreshold       float64
	HighThreshold      float64
	ObjecttypeId       uint8
	Name1              string
	Name2              string
}

func convertFlappingRows(
	env string, envId, endpointId icingadbTypes.Binary,
	selectCache func(dest interface{}, query string, args ...interface{}), _ *sqlx.Tx, ir interface{},
) (icingaDbUpdates, icingaDbInserts [][]interface{}, checkpoint interface{}) {
	idoRows := ir.([]flappingRow)
	if len(idoRows) < 1 {
		return
	}

	var cached []struct {
		HistoryId     uint64
		EventTime     int64
		EventTimeUsec uint32
	}
	selectCache(
		&cached, "SELECT history_id, event_time, event_time_usec FROM end_start_time WHERE history_id BETWEEN ? AND ?",
		idoRows[0].FlappinghistoryId, idoRows[len(idoRows)-1].FlappinghistoryId,
	)

	// Needed for start time (see below).
	cachedById := make(map[uint64]icingadbTypes.UnixMilli, len(cached))
	for _, c := range cached {
		cachedById[c.HistoryId] = convertTime(c.EventTime, c.EventTimeUsec)
	}

	var flappingHistory, flappingHistoryUpdates, allHistory []interface{}
	for _, row := range idoRows {
		ts := convertTime(row.EventTime, row.EventTimeUsec)

		// Needed for ID (see below).
		var start icingadbTypes.UnixMilli
		if row.EventType == 1001 { // end
			var ok bool
			start, ok = cachedById[row.FlappinghistoryId]

			if !ok {
				continue
			}
		} else {
			start = ts
		}

		name := row.Name1
		if row.Name2 != "" {
			name += "!" + row.Name2
		}

		typ := objectTypes[row.ObjecttypeId]
		hostId := calcObjectId(env, row.Name1)
		serviceId := calcServiceId(env, row.Name1, row.Name2)
		startTime := float64(utils.UnixMilli(start.Time()))
		flappingHistoryId := hashAny([]interface{}{env, name, startTime})

		if row.EventType == 1001 { // end
			// The start counterpart should already have been inserted.
			flappingHistoryUpdates = append(flappingHistoryUpdates, &FlappingEnd{flappingHistoryId, ts})

			h := &history.HistoryFlapping{
				HistoryMeta: history.HistoryMeta{
					HistoryEntity: history.HistoryEntity{
						Id: hashAny([]interface{}{env, "flapping_end", name, startTime}),
					},
					EnvironmentId: envId,
					EndpointId:    endpointId,
					ObjectType:    typ,
					HostId:        hostId,
					ServiceId:     serviceId,
					EventType:     "flapping_end",
				},
				FlappingHistoryId: flappingHistoryId,
				StartTime:         start,
				EndTime:           ts,
			}

			h.EventTime.History = h
			allHistory = append(allHistory, h)
		} else { // end
			flappingHistory = append(flappingHistory, &history.FlappingHistory{
				EntityWithoutChecksum: v1.EntityWithoutChecksum{
					IdMeta: v1.IdMeta{Id: flappingHistoryId},
				},
				HistoryTableMeta: history.HistoryTableMeta{
					EnvironmentId: envId,
					EndpointId:    endpointId,
					ObjectType:    typ,
					HostId:        hostId,
					ServiceId:     serviceId,
				},
				FlappingHistoryUpserter: history.FlappingHistoryUpserter{
					FlappingThresholdLow:  float32(row.LowThreshold),
					FlappingThresholdHigh: float32(row.HighThreshold),
				},
				StartTime:               start,
				PercentStateChangeStart: icingadbTypes.Float{NullFloat64: row.PercentStateChange},
			})

			h := &history.HistoryFlapping{
				HistoryMeta: history.HistoryMeta{
					HistoryEntity: history.HistoryEntity{
						Id: hashAny([]interface{}{env, "flapping_start", name, startTime}),
					},
					EnvironmentId: envId,
					EndpointId:    endpointId,
					ObjectType:    typ,
					HostId:        hostId,
					ServiceId:     serviceId,
					EventType:     "flapping_start",
				},
				FlappingHistoryId: flappingHistoryId,
				StartTime:         start,
			}

			h.EventTime.History = h
			allHistory = append(allHistory, h)
		}

		checkpoint = row.FlappinghistoryId
	}

	icingaDbUpdates = [][]interface{}{flappingHistoryUpdates}
	icingaDbInserts = [][]interface{}{flappingHistory, allHistory}
	return
}

const notificationMigrationQuery = "SELECT n.notification_id, n.notification_reason, " +
	"UNIX_TIMESTAMP(n.end_time) end_time, n.end_time_usec, n.state, IFNULL(n.output, '') output, " +
	"n.long_output, n.contacts_notified, o.objecttype_id, o.name1, IFNULL(o.name2, '') name2 " +
	"FROM icinga_notifications n USE INDEX (PRIMARY) " +
	"INNER JOIN icinga_objects o ON o.object_id=n.object_id " +
	"WHERE n.notification_id <= :cache_limit AND " +
	"n.notification_id > :checkpoint " + // where we were interrupted
	"ORDER BY n.notification_id " + // allows computeProgress() not to check all IDO rows for whether migrated
	"LIMIT :bulk"

// notificationTypes maps IDO values to Icinga DB ones.
var notificationTypes = map[uint8]icingadbTypes.NotificationType{5: 1, 6: 2, 7: 4, 8: 8, 1: 16, 2: 128, 3: 256}

type notificationRow = struct {
	NotificationId     uint64
	NotificationReason uint8
	EndTime            int64
	EndTimeUsec        uint32
	State              uint8
	Output             string
	LongOutput         sql.NullString
	ContactsNotified   uint16
	ObjecttypeId       uint8
	Name1              string
	Name2              string
}

func convertNotificationRows(
	env string, envId, endpointId icingadbTypes.Binary,
	selectCache func(dest interface{}, query string, args ...interface{}), ido *sqlx.Tx, ir interface{},
) (_, icingaDbInserts [][]interface{}, checkpoint interface{}) {
	idoRows := ir.([]notificationRow)
	if len(idoRows) < 1 {
		return
	}

	var cached []struct {
		HistoryId         uint64
		PreviousHardState uint8
	}
	selectCache(
		&cached, "SELECT history_id, previous_hard_state FROM previous_hard_state WHERE history_id BETWEEN ? AND ?",
		idoRows[0].NotificationId, idoRows[len(idoRows)-1].NotificationId,
	)

	cachedById := make(map[uint64]uint8, len(cached))
	for _, c := range cached {
		cachedById[c.HistoryId] = c.PreviousHardState
	}

	var contacts []struct {
		NotificationId uint64
		Name1          string
	}

	{
		const query = "SELECT c.notification_id, o.name1 FROM icinga_contactnotifications c " +
			"INNER JOIN icinga_objects o ON o.object_id=c.contact_object_id WHERE c.notification_id BETWEEN ? AND ?"

		err := ido.Select(&contacts, query, idoRows[0].NotificationId, idoRows[len(idoRows)-1].NotificationId)
		if err != nil {
			log.With("query", query).Fatalf("%+v", errors.Wrap(err, "can't perform query"))
		}
	}

	contactsById := map[uint64]map[string]struct{}{}
	for _, contact := range contacts {
		perId, ok := contactsById[contact.NotificationId]
		if !ok {
			perId = map[string]struct{}{}
			contactsById[contact.NotificationId] = perId
		}

		perId[contact.Name1] = struct{}{}
	}

	var notificationHistory, userNotificationHistory, allHistory []interface{}
	for _, row := range idoRows {
		previousHardState, ok := cachedById[row.NotificationId]
		if !ok {
			continue
		}

		// The IDO tracks only sent notifications, but not notification config objects. We have to improvise.
		name := strings.Join(
			[]string{row.Name1, row.Name2, "migrated from IDO", strconv.FormatUint(row.NotificationId, 36)}, "!",
		)

		var nt icingadbTypes.NotificationType
		if row.NotificationReason == 0 {
			if row.State == 0 {
				nt = 64 // recovery
			} else {
				nt = 32 // problem
			}
		} else {
			nt = notificationTypes[row.NotificationReason]
		}

		ntEnum, err := nt.Value()
		if err != nil {
			// Programming error
			panic(err)
		}

		ts := convertTime(row.EndTime, row.EndTimeUsec)
		tsMilli := float64(utils.UnixMilli(ts.Time()))
		notificationHistoryId := hashAny([]interface{}{env, name, ntEnum, tsMilli})
		id := hashAny([]interface{}{env, "notification", name, ntEnum, tsMilli})
		typ := objectTypes[row.ObjecttypeId]
		hostId := calcObjectId(env, row.Name1)
		serviceId := calcServiceId(env, row.Name1, row.Name2)

		text := row.Output
		if row.LongOutput.Valid {
			text += "\n\n" + row.LongOutput.String
		}

		notificationHistory = append(notificationHistory, &history.NotificationHistory{
			HistoryTableEntity: history.HistoryTableEntity{
				EntityWithoutChecksum: v1.EntityWithoutChecksum{
					IdMeta: v1.IdMeta{Id: notificationHistoryId},
				},
			},
			HistoryTableMeta: history.HistoryTableMeta{
				EnvironmentId: envId,
				EndpointId:    endpointId,
				ObjectType:    typ,
				HostId:        hostId,
				ServiceId:     serviceId,
			},
			NotificationId:    calcObjectId(env, name),
			Type:              nt,
			SendTime:          ts,
			State:             row.State,
			PreviousHardState: previousHardState,
			Author:            "-",
			Text:              text,
			UsersNotified:     row.ContactsNotified,
		})

		allHistory = append(allHistory, &history.HistoryNotification{
			HistoryMeta: history.HistoryMeta{
				HistoryEntity: history.HistoryEntity{Id: id},
				EnvironmentId: envId,
				EndpointId:    endpointId,
				ObjectType:    typ,
				HostId:        hostId,
				ServiceId:     serviceId,
				EventType:     "notification",
			},
			NotificationHistoryId: notificationHistoryId,
			EventTime:             ts,
		})

		for contact := range contactsById[row.NotificationId] {
			userId := calcObjectId(env, contact)

			userNotificationHistory = append(userNotificationHistory, &history.UserNotificationHistory{
				EntityWithoutChecksum: v1.EntityWithoutChecksum{
					IdMeta: v1.IdMeta{
						Id: utils.Checksum(append(append([]byte(nil), notificationHistoryId...), userId...)),
					},
				},
				EnvironmentMeta:       v1.EnvironmentMeta{EnvironmentId: envId},
				NotificationHistoryId: id,
				UserId:                userId,
			})
		}

		checkpoint = row.NotificationId
	}

	icingaDbInserts = [][]interface{}{notificationHistory, userNotificationHistory, allHistory}
	return
}

const stateMigrationQuery = "SELECT sh.statehistory_id, UNIX_TIMESTAMP(sh.state_time) state_time, " +
	"sh.state_time_usec, sh.state, sh.state_type, sh.current_check_attempt, " +
	"sh.max_check_attempts, sh.last_state, sh.last_hard_state, sh.output, sh.long_output, " +
	"sh.check_source, o.objecttype_id, o.name1, IFNULL(o.name2, '') name2 " +
	"FROM icinga_statehistory sh USE INDEX (PRIMARY) " +
	"INNER JOIN icinga_objects o ON o.object_id=sh.object_id " +
	"WHERE sh.statehistory_id <= :cache_limit AND " +
	"sh.statehistory_id > :checkpoint " + // where we were interrupted
	"ORDER BY sh.statehistory_id " + // allows computeProgress() not to check all IDO rows for whether migrated
	"LIMIT :bulk"

type stateRow = struct {
	StatehistoryId      uint64
	StateTime           int64
	StateTimeUsec       uint32
	State               uint8
	StateType           uint8
	CurrentCheckAttempt uint16
	MaxCheckAttempts    uint16
	LastState           uint8
	LastHardState       uint8
	Output              sql.NullString
	LongOutput          sql.NullString
	CheckSource         sql.NullString
	ObjecttypeId        uint8
	Name1               string
	Name2               string
}

func convertStateRows(
	env string, envId, endpointId icingadbTypes.Binary,
	selectCache func(dest interface{}, query string, args ...interface{}), _ *sqlx.Tx, ir interface{},
) (_, icingaDbInserts [][]interface{}, checkpoint interface{}) {
	idoRows := ir.([]stateRow)
	if len(idoRows) < 1 {
		return
	}

	var cached []struct {
		HistoryId         uint64
		PreviousHardState uint8
	}
	selectCache(
		&cached, "SELECT history_id, previous_hard_state FROM previous_hard_state WHERE history_id BETWEEN ? AND ?",
		idoRows[0].StatehistoryId, idoRows[len(idoRows)-1].StatehistoryId,
	)

	cachedById := make(map[uint64]uint8, len(cached))
	for _, c := range cached {
		cachedById[c.HistoryId] = c.PreviousHardState
	}

	var stateHistory, allHistory []interface{}
	for _, row := range idoRows {
		previousHardState, ok := cachedById[row.StatehistoryId]
		if !ok {
			continue
		}

		name := strings.Join([]string{row.Name1, row.Name2}, "!")
		ts := convertTime(row.StateTime, row.StateTimeUsec)
		tsMilli := float64(utils.UnixMilli(ts.Time()))
		stateHistoryId := hashAny([]interface{}{env, name, tsMilli})
		id := hashAny([]interface{}{env, "state_change", name, tsMilli})
		typ := objectTypes[row.ObjecttypeId]
		hostId := calcObjectId(env, row.Name1)
		serviceId := calcServiceId(env, row.Name1, row.Name2)

		stateHistory = append(stateHistory, &history.StateHistory{
			HistoryTableEntity: history.HistoryTableEntity{
				EntityWithoutChecksum: v1.EntityWithoutChecksum{
					IdMeta: v1.IdMeta{Id: stateHistoryId},
				},
			},
			HistoryTableMeta: history.HistoryTableMeta{
				EnvironmentId: envId,
				EndpointId:    endpointId,
				ObjectType:    typ,
				HostId:        hostId,
				ServiceId:     serviceId,
			},
			EventTime:         ts,
			StateType:         icingadbTypes.StateType(row.StateType),
			SoftState:         row.State,
			HardState:         row.LastHardState,
			PreviousSoftState: row.LastState,
			PreviousHardState: previousHardState,
			Attempt:           uint8(row.CurrentCheckAttempt),
			Output:            icingadbTypes.String{NullString: row.Output},
			LongOutput:        icingadbTypes.String{NullString: row.LongOutput},
			MaxCheckAttempts:  uint32(row.MaxCheckAttempts),
			CheckSource:       icingadbTypes.String{NullString: row.CheckSource},
		})

		allHistory = append(allHistory, &history.HistoryState{
			HistoryMeta: history.HistoryMeta{
				HistoryEntity: history.HistoryEntity{Id: id},
				EnvironmentId: envId,
				EndpointId:    endpointId,
				ObjectType:    typ,
				HostId:        hostId,
				ServiceId:     serviceId,
				EventType:     "state_change",
			},
			StateHistoryId: stateHistoryId,
			EventTime:      ts,
		})

		checkpoint = row.StatehistoryId
	}

	icingaDbInserts = [][]interface{}{stateHistory, allHistory}
	return
}
