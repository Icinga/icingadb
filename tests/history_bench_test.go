package icingadb_test

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/icinga/icinga-testing/utils"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
	"time"
)

func BenchmarkHistory(b *testing.B) {
	for _, numComments := range []int64{100_000, 200_000} {
		b.Run(fmt.Sprintf("%d-Comments", numComments), func(b *testing.B) {
			b.StopTimer()

			for i := 0; i < b.N; i++ {
				benchmarkHistory(b, numComments)
			}
		})
	}

}

func benchmarkHistory(b *testing.B, numComments int64) {
	m := it.MysqlDatabase()
	defer m.Cleanup()
	m.ImportIcingaDbSchema()

	r := it.RedisServer()
	defer r.Cleanup()
	n := it.Icinga2Node("master")
	defer n.Cleanup()
	n.EnableIcingaDb(r)
	err := n.Reload()
	require.NoError(b, err, "icinga2 should reload without error")

	db, err := sqlx.Connect("mysql", m.DSN())
	require.NoError(b, err, "connecting to mysql")
	defer func() { _ = db.Close() }()

	redisClient := r.Open()
	defer func() { _ = redisClient.Close() }()

	client := n.ApiClient()

	hostname := utils.UniqueName(b, "host")
	client.CreateHost(b, hostname, map[string]interface{}{
		"attrs": map[string]interface{}{
			"enable_active_checks":  false,
			"enable_passive_checks": true,
			"check_command":         "dummy",
		},
	})

	baseTime := time.Now().Add(time.Duration(-numComments) * time.Second)
	for i := int64(0); i < numComments; i++ {
		redisClient.XAdd(context.Background(), &redis.XAddArgs{
			Stream: "icinga:history:stream:comment",
			Values: map[string]string{
				"comment_id":     utils.RandomString(32),
				"environment_id": "da39a3ee5e6b4b0d3255bfef95601890afd80709",
				"host_id":        "05d7e9c12104a1e8851a871d2f78e25b8c3d9eae",
				"entry_time":     strconv.FormatInt(baseTime.Add(time.Duration(i)*time.Second).UnixMilli(), 10),
				"author":         utils.RandomString(8),
				"comment":        utils.RandomString(8),
				"entry_type":     "1",
				"is_persistent":  "0",
				"is_sticky":      "0",
				"event_id":       uuid.New().String(),
				"event_type":     "comment_add",
				"object_type":    "service",
				"service_id":     "98fe4a1696c4804c75ff5c0e76f1e79ef855c634",
				"endpoint_id":    "05d7e9c12104a1e8851a871d2f78e25b8c3d9eae",
			},
		})
	}

	pendingCount := func() int64 {
		result, err := redisClient.XInfoStream(context.Background(), "icinga:history:stream:comment").Result()
		require.NoError(b, err, "XINFO should not fail")
		return result.Length
	}

	writtenCount := func() int64 {
		var count int64
		err := db.Get(&count, "SELECT COUNT(*) FROM comment_history")
		require.NoError(b, err, "SELECT COUNT(*) should not fail")
		return count
	}

	lastPending := pendingCount()
	b.Logf("current stream length: %d", lastPending)

	b.StartTimer()
	idb := it.IcingaDbInstance(r, m)
	defer idb.Cleanup()

	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	logTicker := time.NewTicker(1 * time.Second)
	defer logTicker.Stop()
	timeout := time.NewTicker(5 * time.Minute)
	defer timeout.Stop()

loop:
	for {
		select {
		case <-ticker.C:
			if pendingCount() == 0 && writtenCount() >= numComments {
				break loop
			}
		case <-logTicker.C:
			if p := pendingCount(); p > 0 {
				b.Logf("last second: %d, pending: %d", lastPending-p, p)
				lastPending = p
			} else {
				logTicker.Stop()
			}
		case <-timeout.C:
			b.Fatal("did not drain stream in time")
		}
	}

	b.StopTimer()
}
