package telemetry

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/icinga/icingadb/pkg/periodic"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"regexp"
	"runtime/metrics"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// ha represents icingadb.HA to avoid import cycles.
type ha interface {
	State() (weResponsibleMilli int64, weResponsible, otherResponsible bool)
}

type SuccessfulSync struct {
	FinishMilli   int64
	DurationMilli int64
}

type DbConnErr struct {
	Message    string
	SinceMilli int64
}

// CurrentDbConnErr is to be updated by the DB connector.
var CurrentDbConnErr com.Atomic[DbConnErr]

// OngoingSyncStartMilli is to be updated by the main() function.
var OngoingSyncStartMilli int64

// LastSuccessfulSync is to be updated by the main() function.
var LastSuccessfulSync com.Atomic[SuccessfulSync]

var boolToStr = map[bool]string{false: "0", true: "1"}
var startTime = time.Now().UnixMilli()

// StartHeartbeat periodically writes heartbeats to Redis for being monitored by Icinga 2.
func StartHeartbeat(
	ctx context.Context, client *icingaredis.Client, logger *logging.Logger, ha ha, heartbeat *icingaredis.Heartbeat,
) {
	goMetrics := NewGoMetrics()

	const interval = time.Second

	var lastErr string
	var silenceUntil time.Time

	periodic.Start(ctx, interval, func(tick periodic.Tick) {
		heartbeat := heartbeat.LastReceived()
		responsibleTsMilli, responsible, otherResponsible := ha.State()
		ongoingSyncStart := atomic.LoadInt64(&OngoingSyncStartMilli)
		sync, _ := LastSuccessfulSync.Load()
		dbConnErr, _ := CurrentDbConnErr.Load()
		now := time.Now()

		values := map[string]string{
			"version":                 internal.Version.Version,
			"time":                    strconv.FormatInt(now.UnixMilli(), 10),
			"start-time":              strconv.FormatInt(startTime, 10),
			"error":                   dbConnErr.Message,
			"error-since":             strconv.FormatInt(dbConnErr.SinceMilli, 10),
			"performance-data":        goMetrics.PerformanceData(),
			"last-heartbeat-received": strconv.FormatInt(heartbeat, 10),
			"ha-responsible":          boolToStr[responsible],
			"ha-responsible-ts":       strconv.FormatInt(responsibleTsMilli, 10),
			"ha-other-responsible":    boolToStr[otherResponsible],
			"sync-ongoing-since":      strconv.FormatInt(ongoingSyncStart, 10),
			"sync-success-finish":     strconv.FormatInt(sync.FinishMilli, 10),
			"sync-success-duration":   strconv.FormatInt(sync.DurationMilli, 10),
		}

		ctx, cancel := context.WithDeadline(ctx, tick.Time.Add(interval))
		defer cancel()

		cmd := client.XAdd(ctx, &redis.XAddArgs{
			Stream: "icingadb:telemetry:heartbeat",
			MaxLen: 1,
			Values: values,
		})
		if err := cmd.Err(); err != nil && !utils.IsContextCanceled(err) && !errors.Is(err, context.DeadlineExceeded) {
			logw := logger.Debugw
			currentErr := err.Error()

			if currentErr != lastErr || now.After(silenceUntil) {
				logw = logger.Warnw
				lastErr = currentErr
				silenceUntil = now.Add(time.Minute)
			}

			logw("Can't update own heartbeat", zap.Error(icingaredis.WrapCmdErr(cmd)))
		} else {
			lastErr = ""
			silenceUntil = time.Time{}
		}
	})
}

type goMetrics struct {
	names   []string
	units   []string
	samples []metrics.Sample
}

func NewGoMetrics() *goMetrics {
	m := &goMetrics{}

	forbiddenRe := regexp.MustCompile(`\W`)

	for _, d := range metrics.All() {
		switch d.Kind {
		case metrics.KindUint64, metrics.KindFloat64:
			name := "go_" + strings.TrimLeft(forbiddenRe.ReplaceAllString(d.Name, "_"), "_")

			unit := ""
			if strings.HasSuffix(d.Name, ":bytes") {
				unit = "B"
			} else if strings.HasSuffix(d.Name, ":seconds") {
				unit = "s"
			} else if d.Cumulative {
				unit = "c"
			}

			m.names = append(m.names, name)
			m.units = append(m.units, unit)
			m.samples = append(m.samples, metrics.Sample{Name: d.Name})
		}
	}

	return m
}

func (g *goMetrics) PerformanceData() string {
	metrics.Read(g.samples)

	var buf strings.Builder

	for i, sample := range g.samples {
		if i > 0 {
			buf.WriteByte(' ')
		}

		switch sample.Value.Kind() {
		case metrics.KindUint64:
			_, _ = fmt.Fprintf(&buf, "%s=%d%s", g.names[i], sample.Value.Uint64(), g.units[i])
		case metrics.KindFloat64:
			_, _ = fmt.Fprintf(&buf, "%s=%f%s", g.names[i], sample.Value.Float64(), g.units[i])
		}
	}

	return buf.String()
}
