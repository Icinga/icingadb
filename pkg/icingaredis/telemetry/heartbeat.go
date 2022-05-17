package telemetry

import (
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/icinga/icingadb/pkg/periodic"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"runtime/metrics"
	"strconv"
	"sync/atomic"
	"time"
)

// ha represents icingadb.HA to avoid import cycles.
type ha interface {
	State() (weResponsibleMilli int64, weResponsible, otherResponsible bool)
}

// jsonMetricValue allows JSON-encoding a metrics.Value.
type jsonMetricValue struct {
	v *metrics.Value
}

// MarshalJSON implements the json.Marshaler interface.
func (jmv jsonMetricValue) MarshalJSON() ([]byte, error) {
	switch kind := jmv.v.Kind(); kind {
	case metrics.KindUint64:
		return json.Marshal(jmv.v.Uint64())
	case metrics.KindFloat64:
		return json.Marshal(jmv.v.Float64())
	default:
		return nil, errors.Errorf("can't JSON-encode Go metric value of kind %d", int(kind))
	}
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
	allMetrics := metrics.All()
	samples := make([]metrics.Sample, 0, len(allMetrics))
	cumulative := map[string]jsonMetricValue{}
	notCumulative := map[string]jsonMetricValue{}
	byCumulative := map[bool]map[string]jsonMetricValue{true: cumulative, false: notCumulative}
	mtrcs := map[string]map[string]jsonMetricValue{"cumulative": cumulative, "not-cumulative": notCumulative}

	for _, m := range allMetrics {
		switch m.Kind {
		case metrics.KindUint64, metrics.KindFloat64:
			samples = append(samples, metrics.Sample{Name: m.Name})
			byCumulative[m.Cumulative][m.Name] = jsonMetricValue{&samples[len(samples)-1].Value}
		}
	}

	const interval = time.Second

	var lastErr string
	var silenceUntil time.Time

	periodic.Start(ctx, interval, func(tick periodic.Tick) {
		metrics.Read(samples)

		heartbeat := heartbeat.LastReceived()
		responsibleTsMilli, responsible, otherResponsible := ha.State()
		ongoingSyncStart := atomic.LoadInt64(&OngoingSyncStartMilli)
		sync, _ := LastSuccessfulSync.Load()
		dbConnErr, _ := CurrentDbConnErr.Load()
		now := time.Now()

		var metricsStr string
		if metricsBytes, err := json.Marshal(mtrcs); err == nil {
			metricsStr = string(metricsBytes)
		} else {
			metricsStr = "{}"
			logger.Warnw("Can't JSON-encode Go metrics", zap.Error(errors.WithStack(err)))
		}

		values := map[string]string{
			"general:version":         internal.Version.Version,
			"general:time":            strconv.FormatInt(now.UnixMilli(), 10),
			"general:start-time":      strconv.FormatInt(startTime, 10),
			"general:err":             dbConnErr.Message,
			"general:err-since":       strconv.FormatInt(dbConnErr.SinceMilli, 10),
			"go:metrics":              metricsStr,
			"heartbeat:last-received": strconv.FormatInt(heartbeat, 10),
			"ha:responsible":          boolToStr[responsible],
			"ha:responsible-ts":       strconv.FormatInt(responsibleTsMilli, 10),
			"ha:other-responsible":    boolToStr[otherResponsible],
			"sync:ongoing-since":      strconv.FormatInt(ongoingSyncStart, 10),
			"sync:success-finish":     strconv.FormatInt(sync.FinishMilli, 10),
			"sync:success-duration":   strconv.FormatInt(sync.DurationMilli, 10),
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

// Assert interface compliance.
var _ json.Marshaler = jsonMetricValue{}
