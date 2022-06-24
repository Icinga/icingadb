package telemetry

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/icinga/icingadb/pkg/periodic"
	"github.com/icinga/icingadb/pkg/utils"
	"go.uber.org/zap"
	"strconv"
	"time"
)

var Stats struct {
	// Config & co. are to be increased by the T sync once for every T object synced.
	Config, State, History, Overdue, HistoryCleanup com.Counter
}

// WriteStats periodically forwards Stats to Redis for being monitored by Icinga 2.
func WriteStats(ctx context.Context, client *icingaredis.Client, logger *logging.Logger) {
	counters := map[string]*com.Counter{
		"config_sync":     &Stats.Config,
		"state_sync":      &Stats.State,
		"history_sync":    &Stats.History,
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
				logger.Warnw("Can't update own stats", zap.Error(icingaredis.WrapCmdErr(cmd)))
			}
		}
	})
}
