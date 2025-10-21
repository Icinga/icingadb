package telemetry

import (
	"context"
	"fmt"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/periodic"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icinga-go-library/utils"
	"go.uber.org/zap"
	"iter"
	"strconv"
	"sync"
	"time"
)

// StatsKeeper holds multiple [com.Counter] values by name, to be used for statistics in WriteStats.
type StatsKeeper struct {
	m sync.Map
}

// Get or create a [com.Counter] by its name.
func (statsKeeper *StatsKeeper) Get(key string) *com.Counter {
	ctrAny, _ := statsKeeper.m.LoadOrStore(key, &com.Counter{})

	ctr, ok := ctrAny.(*com.Counter)
	if !ok {
		// Should not happen unless someone messes with the internal map.
		panic(fmt.Sprintf(
			"StatsKeeper.Get(%q) returned something of type %T, not *com.Counter",
			key, ctrAny))
	}

	return ctr
}

// Iterator over all keys and their [com.Counter].
func (statsKeeper *StatsKeeper) Iterator() iter.Seq2[string, *com.Counter] {
	return func(yield func(string, *com.Counter) bool) {
		statsKeeper.m.Range(func(keyAny, ctrAny any) bool {
			key, keyOk := keyAny.(string)
			ctr, ctrOk := ctrAny.(*com.Counter)
			if !keyOk || !ctrOk {
				// Should not happen unless someone messes with the internal map.
				panic(fmt.Sprintf(
					"iterating StatsKeeper on key %q got types (%T, %T), not (string, *com.Counter)",
					keyAny, keyAny, ctrAny))
			}

			return yield(key, ctr)
		})
	}
}

// Stats is the singleton StatsKeeper to be used to access a [com.Counter].
var Stats = &StatsKeeper{}

// Keys for different well known Stats entries.
const (
	StatConfig         = "config_sync"
	StatState          = "state_sync"
	StatHistory        = "history_sync"
	StatOverdue        = "overdue_sync"
	StatHistoryCleanup = "history_cleanup"
)

// WriteStats periodically forwards Stats to Redis for being monitored by Icinga 2.
func WriteStats(ctx context.Context, client *redis.Client, logger *logging.Logger) {
	periodic.Start(ctx, time.Second, func(_ periodic.Tick) {
		var data []string
		for kind, counter := range Stats.Iterator() {
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
