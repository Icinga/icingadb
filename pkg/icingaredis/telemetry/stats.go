package telemetry

import (
	"context"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/periodic"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icinga-go-library/utils"
	"go.uber.org/zap"
	"strconv"
	"time"
)

var Stats struct {
	// Config & co. are to be increased by the T sync once for every T object synced.
	Config         com.Counter
	State          com.Counter
	History        com.Counter
	Callback       com.Counter
	Overdue        com.Counter
	HistoryCleanup com.Counter
}

// WriteStats periodically forwards Stats to Redis for being monitored by Icinga 2.
func WriteStats(ctx context.Context, client *redis.Client, logger *logging.Logger) {
	counters := map[string]*com.Counter{
		"config_sync":     &Stats.Config,
		"state_sync":      &Stats.State,
		"history_sync":    &Stats.History,
		"callback_sync":   &Stats.Callback,
		"overdue_sync":    &Stats.Overdue,
		"history_cleanup": &Stats.HistoryCleanup,
	}

	periodic.Start(ctx, time.Second, func(_ periodic.Tick) {
		var data []string
		for kind, counter := range counters {
			if cnt := counter.Reset(); cnt > 0 {
				data = append(data, kind, strconv.FormatUint(cnt, 10))
			}
		}

		if data != nil {
			cmd := client.XAdd(ctx, &redis.XAddArgs{
				Stream: "icingadb:telemetry:stats",
				MaxLen: 15 * 60,
				Approx: true,
				Values: data,
			})
			if err := cmd.Err(); err != nil && !utils.IsContextCanceled(err) {
				logger.Warnw("Can't update own stats", zap.Error(redis.WrapCmdErr(cmd)))
			}
		}
	})
}
