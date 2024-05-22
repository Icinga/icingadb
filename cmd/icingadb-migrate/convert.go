package main

import (
	"database/sql"
	_ "embed"
	"github.com/icinga/icingadb/pkg/database"
	icingadbTypes "github.com/icinga/icingadb/pkg/icingadb/types"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/icinga/icingadb/pkg/icingadb/v1/history"
	"github.com/icinga/icingadb/pkg/types"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"strconv"
	"strings"
	"time"
)

//go:embed embed/comment_query.sql
var commentMigrationQuery string

//go:embed embed/downtime_query.sql
var downtimeMigrationQuery string

//go:embed embed/flapping_query.sql
var flappingMigrationQuery string

//go:embed embed/notification_query.sql
var notificationMigrationQuery string

//go:embed embed/state_query.sql
var stateMigrationQuery string

type commentRow = struct {
	CommenthistoryId uint64
	EntryTime        sql.NullInt64
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
	env string, envId types.Binary,
	_ func(interface{}, string, ...interface{}), _ *sqlx.Tx, idoRows []commentRow,
) (stages []icingaDbOutputStage, checkpoint any) {
	var commentHistory, acknowledgementHistory, allHistoryComment, allHistoryAck []database.Entity

	for _, row := range idoRows {
		checkpoint = row.CommenthistoryId

		if !row.EntryTime.Valid {
			continue
		}

		typ := objectTypes[row.ObjecttypeId]
		hostId := calcObjectId(env, row.Name1)
		serviceId := calcServiceId(env, row.Name1, row.Name2)

		switch row.EntryType {
		case 1: // user
			id := calcObjectId(env, row.Name)
			entryTime := convertTime(row.EntryTime.Int64, row.EntryTimeUsec)
			removeTime := convertTime(row.DeletionTime, row.DeletionTimeUsec)
			expireTime := convertTime(row.ExpirationTime, 0)

			commentHistory = append(commentHistory, &history.CommentHistory{
				CommentHistoryEntity: history.CommentHistoryEntity{CommentId: id},
				HistoryTableMeta: history.HistoryTableMeta{
					EnvironmentId: envId,
					ObjectType:    typ,
					HostId:        hostId,
					ServiceId:     serviceId,
				},
				CommentHistoryUpserter: history.CommentHistoryUpserter{
					RemoveTime:     removeTime,
					HasBeenRemoved: types.Bool{Bool: !removeTime.Time().IsZero(), Valid: true},
				},
				EntryTime:    entryTime,
				Author:       row.AuthorName,
				Comment:      row.CommentData,
				EntryType:    icingadbTypes.CommentType(row.EntryType),
				IsPersistent: types.Bool{Bool: row.IsPersistent != 0, Valid: true},
				IsSticky:     types.Bool{Bool: false, Valid: true},
				ExpireTime:   expireTime,
			})

			h1 := &history.HistoryComment{
				HistoryMeta: history.HistoryMeta{
					HistoryEntity: history.HistoryEntity{Id: hashAny([]string{env, "comment_add", row.Name})},
					EnvironmentId: envId,
					ObjectType:    typ,
					HostId:        hostId,
					ServiceId:     serviceId,
					EventType:     "comment_add",
				},
				CommentHistoryId: id,
				EntryTime:        entryTime,
			}

			h1.EventTime.History = h1
			allHistoryComment = append(allHistoryComment, h1)

			if !removeTime.Time().IsZero() { // remove
				h2 := &history.HistoryComment{
					HistoryMeta: history.HistoryMeta{
						HistoryEntity: history.HistoryEntity{Id: hashAny([]string{env, "comment_remove", row.Name})},
						EnvironmentId: envId,
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
				allHistoryComment = append(allHistoryComment, h2)
			}
		case 4: // ack
			name := row.Name1
			if row.Name2 != "" {
				name += "!" + row.Name2
			}

			setTime := convertTime(row.EntryTime.Int64, row.EntryTimeUsec)
			setTs := float64(setTime.Time().UnixMilli())
			clearTime := convertTime(row.DeletionTime, row.DeletionTimeUsec)
			acknowledgementHistoryId := hashAny([]any{env, name, setTs})

			acknowledgementHistory = append(acknowledgementHistory, &history.AcknowledgementHistory{
				EntityWithoutChecksum: v1.EntityWithoutChecksum{
					IdMeta: v1.IdMeta{Id: acknowledgementHistoryId},
				},
				HistoryTableMeta: history.HistoryTableMeta{
					EnvironmentId: envId,
					ObjectType:    typ,
					HostId:        hostId,
					ServiceId:     serviceId,
				},
				AckHistoryUpserter: history.AckHistoryUpserter{ClearTime: clearTime},
				SetTime:            setTime,
				Author:             types.MakeString(row.AuthorName),
				Comment:            types.MakeString(row.CommentData),
				ExpireTime:         convertTime(row.ExpirationTime, 0),
				IsPersistent: types.Bool{
					Bool:  row.IsPersistent != 0,
					Valid: true,
				},
			})

			h1 := &history.HistoryAck{
				HistoryMeta: history.HistoryMeta{
					HistoryEntity: history.HistoryEntity{
						Id: hashAny([]any{env, "ack_set", name, setTs}),
					},
					EnvironmentId: envId,
					ObjectType:    typ,
					HostId:        hostId,
					ServiceId:     serviceId,
					EventType:     "ack_set",
				},
				AcknowledgementHistoryId: acknowledgementHistoryId,
				SetTime:                  setTime,
				ClearTime:                clearTime,
			}

			h1.EventTime.History = h1
			allHistoryAck = append(allHistoryAck, h1)

			if !clearTime.Time().IsZero() {
				h2 := &history.HistoryAck{
					HistoryMeta: history.HistoryMeta{
						HistoryEntity: history.HistoryEntity{
							Id: hashAny([]any{env, "ack_clear", name, setTs}),
						},
						EnvironmentId: envId,
						ObjectType:    typ,
						HostId:        hostId,
						ServiceId:     serviceId,
						EventType:     "ack_clear",
					},
					AcknowledgementHistoryId: acknowledgementHistoryId,
					SetTime:                  setTime,
					ClearTime:                clearTime,
				}

				h2.EventTime.History = h2
				allHistoryAck = append(allHistoryAck, h2)
			}
		}
	}

	stages = []icingaDbOutputStage{
		{insert: commentHistory},
		{insert: acknowledgementHistory},
		{insert: allHistoryComment},
		{insert: allHistoryAck},
	}
	return
}

type downtimeRow = struct {
	DowntimehistoryId   uint64
	EntryTime           int64
	AuthorName          string
	CommentData         string
	IsFixed             uint8
	Duration            int64
	ScheduledStartTime  sql.NullInt64
	ScheduledEndTime    int64
	WasStarted          uint8
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
	env string, envId types.Binary,
	_ func(interface{}, string, ...interface{}), _ *sqlx.Tx, idoRows []downtimeRow,
) (stages []icingaDbOutputStage, checkpoint any) {
	var downtimeHistory, allHistory, sla []database.Entity

	for _, row := range idoRows {
		checkpoint = row.DowntimehistoryId

		if !row.ScheduledStartTime.Valid || row.WasStarted == 0 {
			continue
		}

		id := calcObjectId(env, row.Name)
		typ := objectTypes[row.ObjecttypeId]
		hostId := calcObjectId(env, row.Name1)
		serviceId := calcServiceId(env, row.Name1, row.Name2)
		scheduledStart := convertTime(row.ScheduledStartTime.Int64, 0)
		scheduledEnd := convertTime(row.ScheduledEndTime, 0)
		triggerTime := convertTime(row.TriggerTime, 0)
		actualStart := convertTime(row.ActualStartTime, row.ActualStartTimeUsec)
		actualEnd := convertTime(row.ActualEndTime, row.ActualEndTimeUsec)
		var startTime, endTime, cancelTime types.UnixMilli

		if scheduledEnd.Time().IsZero() {
			scheduledEnd = types.UnixMilli(scheduledStart.Time().Add(time.Duration(row.Duration) * time.Second))
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
				ObjectType:    typ,
				HostId:        hostId,
				ServiceId:     serviceId,
			},
			DowntimeHistoryUpserter: history.DowntimeHistoryUpserter{
				HasBeenCancelled: types.Bool{Bool: row.WasCancelled != 0, Valid: true},
				CancelTime:       cancelTime,
			},
			TriggeredById:      calcObjectId(env, row.TriggeredBy),
			EntryTime:          convertTime(row.EntryTime, 0),
			Author:             row.AuthorName,
			Comment:            row.CommentData,
			IsFlexible:         types.Bool{Bool: row.IsFixed == 0, Valid: true},
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
					ObjectType:    typ,
					HostId:        hostId,
					ServiceId:     serviceId,
					EventType:     "downtime_end",
				},
				DowntimeHistoryId: id,
				StartTime:         startTime,
				CancelTime:        cancelTime,
				EndTime:           endTime,
				HasBeenCancelled:  types.Bool{Bool: row.WasCancelled != 0, Valid: true},
			}

			h2.EventTime.History = h2
			allHistory = append(allHistory, h2)
		}

		s := &history.SlaHistoryDowntime{
			DowntimeHistoryEntity: history.DowntimeHistoryEntity{DowntimeId: id},
			HistoryTableMeta: history.HistoryTableMeta{
				EnvironmentId: envId,
				ObjectType:    typ,
				HostId:        hostId,
				ServiceId:     serviceId,
			},
			DowntimeStart:    startTime,
			HasBeenCancelled: types.Bool{Bool: row.WasCancelled != 0, Valid: true},
			CancelTime:       cancelTime,
			EndTime:          endTime,
		}

		s.DowntimeEnd.History = s
		sla = append(sla, s)
	}

	stages = []icingaDbOutputStage{
		{insert: downtimeHistory},
		{insert: allHistory},
		{insert: sla},
	}
	return
}

type flappingRow = struct {
	FlappinghistoryId  uint64
	EventTime          sql.NullInt64
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
	env string, envId types.Binary,
	selectCache func(dest interface{}, query string, args ...interface{}), _ *sqlx.Tx, idoRows []flappingRow,
) (stages []icingaDbOutputStage, checkpoint any) {
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
	cachedById := make(map[uint64]types.UnixMilli, len(cached))
	for _, c := range cached {
		cachedById[c.HistoryId] = convertTime(c.EventTime, c.EventTimeUsec)
	}

	var flappingHistory, flappingHistoryUpserts, allHistory []database.Entity
	for _, row := range idoRows {
		checkpoint = row.FlappinghistoryId

		if !row.EventTime.Valid {
			continue
		}

		ts := convertTime(row.EventTime.Int64, row.EventTimeUsec)

		// Needed for ID (see below).
		var start types.UnixMilli
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
		startTime := float64(start.Time().UnixMilli())
		flappingHistoryId := hashAny([]interface{}{env, name, startTime})

		if row.EventType == 1001 { // end
			// The start counterpart should already have been inserted.
			flappingHistoryUpserts = append(flappingHistoryUpserts, &history.FlappingHistory{
				EntityWithoutChecksum: v1.EntityWithoutChecksum{
					IdMeta: v1.IdMeta{Id: flappingHistoryId},
				},
				HistoryTableMeta: history.HistoryTableMeta{
					EnvironmentId: envId,
					ObjectType:    typ,
					HostId:        hostId,
					ServiceId:     serviceId,
				},
				FlappingHistoryUpserter: history.FlappingHistoryUpserter{
					EndTime:               ts,
					PercentStateChangeEnd: types.Float{NullFloat64: row.PercentStateChange},
					FlappingThresholdLow:  float32(row.LowThreshold),
					FlappingThresholdHigh: float32(row.HighThreshold),
				},
				StartTime: start,
			})

			h := &history.HistoryFlapping{
				HistoryMeta: history.HistoryMeta{
					HistoryEntity: history.HistoryEntity{
						Id: hashAny([]interface{}{env, "flapping_end", name, startTime}),
					},
					EnvironmentId: envId,
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
		} else {
			flappingHistory = append(flappingHistory, &history.FlappingHistory{
				EntityWithoutChecksum: v1.EntityWithoutChecksum{
					IdMeta: v1.IdMeta{Id: flappingHistoryId},
				},
				HistoryTableMeta: history.HistoryTableMeta{
					EnvironmentId: envId,
					ObjectType:    typ,
					HostId:        hostId,
					ServiceId:     serviceId,
				},
				FlappingHistoryUpserter: history.FlappingHistoryUpserter{
					FlappingThresholdLow:  float32(row.LowThreshold),
					FlappingThresholdHigh: float32(row.HighThreshold),
				},
				StartTime:               start,
				PercentStateChangeStart: types.Float{NullFloat64: row.PercentStateChange},
			})

			h := &history.HistoryFlapping{
				HistoryMeta: history.HistoryMeta{
					HistoryEntity: history.HistoryEntity{
						Id: hashAny([]interface{}{env, "flapping_start", name, startTime}),
					},
					EnvironmentId: envId,
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
	}

	stages = []icingaDbOutputStage{
		{insert: flappingHistory},
		{upsert: flappingHistoryUpserts},
		{insert: allHistory},
	}
	return
}

type notificationRow = struct {
	NotificationId     uint64
	NotificationReason uint8
	EndTime            sql.NullInt64
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
	env string, envId types.Binary,
	selectCache func(dest interface{}, query string, args ...interface{}), ido *sqlx.Tx, idoRows []notificationRow,
) (stages []icingaDbOutputStage, checkpoint any) {
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
		var query = ido.Rebind(
			"SELECT c.notification_id, o.name1 FROM icinga_contactnotifications c " +
				"INNER JOIN icinga_objects o ON o.object_id=c.contact_object_id WHERE c.notification_id BETWEEN ? AND ?",
		)

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

	var notificationHistory, userNotificationHistory, allHistory []database.Entity
	for _, row := range idoRows {
		checkpoint = row.NotificationId

		if !row.EndTime.Valid {
			continue
		}

		previousHardState, ok := cachedById[row.NotificationId]
		if !ok {
			continue
		}

		// The IDO tracks only sent notifications, but not notification config objects, nor even their names.
		// We have to improvise. By the way we avoid unwanted collisions between synced and migrated data via "ID"
		// instead of "HOST[!SERVICE]!NOTIFICATION" (ok as this name won't be parsed, but only hashed) and between
		// migrated data itself via the history ID as object name, i.e. one "virtual object" per sent notification.
		name := strconv.FormatUint(row.NotificationId, 10)

		nt := convertNotificationType(row.NotificationReason, row.State)

		ntEnum, err := nt.Value()
		if err != nil {
			continue
		}

		ts := convertTime(row.EndTime.Int64, row.EndTimeUsec)
		tsMilli := float64(ts.Time().UnixMilli())
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
				ObjectType:    typ,
				HostId:        hostId,
				ServiceId:     serviceId,
			},
			NotificationId:    calcObjectId(env, name),
			Type:              nt,
			SendTime:          ts,
			State:             row.State,
			PreviousHardState: previousHardState,
			Text:              types.MakeString(text),
			UsersNotified:     row.ContactsNotified,
		})

		allHistory = append(allHistory, &history.HistoryNotification{
			HistoryMeta: history.HistoryMeta{
				HistoryEntity: history.HistoryEntity{Id: id},
				EnvironmentId: envId,
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
				NotificationHistoryId: notificationHistoryId,
				UserId:                userId,
			})
		}
	}

	stages = []icingaDbOutputStage{
		{insert: notificationHistory},
		{insert: userNotificationHistory},
		{insert: allHistory},
	}
	return
}

// convertNotificationType maps IDO values[1] to Icinga DB ones[2].
//
// [1]: https://github.com/Icinga/icinga2/blob/32c7f7730db154ba0dff5856a8985d125791c/lib/db_ido/dbevents.cpp#L1507-L1524
// [2]: https://github.com/Icinga/icingadb/blob/8f31ac143875498797725adb9bfacf3d4/pkg/types/notification_type.go#L53-L61
func convertNotificationType(notificationReason, state uint8) icingadbTypes.NotificationType {
	switch notificationReason {
	case 0: // state
		if state == 0 {
			return 64 // recovery
		} else {
			return 32 // problem
		}
	case 1: // acknowledgement
		return 16
	case 2: // flapping start
		return 128
	case 3: // flapping end
		return 256
	case 5: // downtime start
		return 1
	case 6: // downtime end
		return 2
	case 7: // downtime removed
		return 4
	case 8: // custom
		return 8
	default: // bad notification type
		return 0
	}
}

type stateRow = struct {
	StatehistoryId      uint64
	StateTime           sql.NullInt64
	StateTimeUsec       uint32
	State               uint8
	StateType           uint8
	CurrentCheckAttempt uint32
	MaxCheckAttempts    uint32
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
	env string, envId types.Binary,
	selectCache func(dest interface{}, query string, args ...interface{}), _ *sqlx.Tx, idoRows []stateRow,
) (stages []icingaDbOutputStage, checkpoint any) {
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

	var stateHistory, allHistory, sla []database.Entity
	for _, row := range idoRows {
		checkpoint = row.StatehistoryId

		if !row.StateTime.Valid {
			continue
		}

		previousHardState, ok := cachedById[row.StatehistoryId]
		if !ok {
			continue
		}

		name := strings.Join([]string{row.Name1, row.Name2}, "!")
		ts := convertTime(row.StateTime.Int64, row.StateTimeUsec)
		tsMilli := float64(ts.Time().UnixMilli())
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
			CheckAttempt:      row.CurrentCheckAttempt,
			Output:            types.String{NullString: row.Output},
			LongOutput:        types.String{NullString: row.LongOutput},
			MaxCheckAttempts:  row.MaxCheckAttempts,
			CheckSource:       types.String{NullString: row.CheckSource},
		})

		allHistory = append(allHistory, &history.HistoryState{
			HistoryMeta: history.HistoryMeta{
				HistoryEntity: history.HistoryEntity{Id: id},
				EnvironmentId: envId,
				ObjectType:    typ,
				HostId:        hostId,
				ServiceId:     serviceId,
				EventType:     "state_change",
			},
			StateHistoryId: stateHistoryId,
			EventTime:      ts,
		})

		if icingadbTypes.StateType(row.StateType) == icingadbTypes.StateHard {
			// only hard state changes are relevant for SLA history, discard all others

			sla = append(sla, &history.SlaHistoryState{
				HistoryTableEntity: history.HistoryTableEntity{
					EntityWithoutChecksum: v1.EntityWithoutChecksum{
						IdMeta: v1.IdMeta{Id: stateHistoryId},
					},
				},
				HistoryTableMeta: history.HistoryTableMeta{
					EnvironmentId: envId,
					ObjectType:    typ,
					HostId:        hostId,
					ServiceId:     serviceId,
				},
				EventTime:         ts,
				StateType:         icingadbTypes.StateType(row.StateType),
				HardState:         row.LastHardState,
				PreviousHardState: previousHardState,
			})
		}
	}

	stages = []icingaDbOutputStage{
		{insert: stateHistory},
		{insert: allHistory},
		{insert: sla},
	}
	return
}
