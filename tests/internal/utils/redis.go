package utils

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"github.com/icinga/icinga-testing/services"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func GetEnvironmentIdFromRedis(t *testing.T, r services.RedisServer) []byte {
	conn := r.Open()
	defer conn.Close()

	heartbeat, err := conn.XRead(context.Background(), &redis.XReadArgs{
		Streams: []string{"icinga:stats", "0"},
		Count:   1,
		Block:   10 * time.Second,
	}).Result()
	require.NoError(t, err, "reading from icinga:stats failed")
	require.NotEmpty(t, heartbeat, "response contains no streams")
	require.NotEmpty(t, heartbeat[0].Messages, "response contains no messages")
	require.Contains(t, heartbeat[0].Messages[0].Values, "icingadb_environment",
		"icinga:stats message misses icingadb_environment")

	var envIdHex string
	err = json.Unmarshal([]byte(heartbeat[0].Messages[0].Values["icingadb_environment"].(string)), &envIdHex)
	require.NoError(t, err, "cannot parse environment ID as a JSON string")

	envId, err := hex.DecodeString(envIdHex)
	require.NoError(t, err, "environment ID is not a hex string")

	return envId
}
