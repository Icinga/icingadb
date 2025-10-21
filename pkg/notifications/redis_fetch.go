package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
	"github.com/redis/go-redis/v9"
)

// fetchHostServiceName retrieves the host and service names from Redis.
//
// It uses either the hostId or/and serviceId to fetch the corresponding names. If both are provided,
// the returned result will contain the host name and the service name accordingly. Otherwise, it will
// only contain the host name.
//
// Internally, it uses the Redis HGet command to fetch the data from the "icinga:host" and "icinga:service" hashes.
// If this operation couldn't be completed within a reasonable time (a hard coded 5 seconds), it will cancel the
// request and return an error indicating that the operation timed out. In case of the serviceId being set, the
// maximum execution time of the Redis HGet commands is 10s (5s for each HGet call).
func (client *Client) fetchHostServiceName(ctx context.Context, hostId, serviceId types.Binary) (*redisLookupResult, error) {
	getNameFromRedis := func(typ, id string) (string, error) {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		var data string
		err := retry.WithBackoff(
			ctx,
			func(ctx context.Context) (err error) {
				data, err = client.redisClient.HGet(ctx, "icinga:"+typ, id).Result()
				return
			},
			retry.Retryable,
			backoff.DefaultBackoff,
			retry.Settings{},
		)
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return "", fmt.Errorf("%s with ID %s not found in Redis", typ, hostId)
			}
			return "", fmt.Errorf("failed to fetch %s with ID %s from Redis: %w", typ, id, err)
		}

		var result struct {
			Name string `json:"name"`
		}

		if err := json.Unmarshal([]byte(data), &result); err != nil {
			return "", fmt.Errorf("failed to unmarshal redis result: %w", err)
		}

		return result.Name, nil
	}

	var result redisLookupResult
	var err error

	result.HostName, err = getNameFromRedis("host", hostId.String())
	if err != nil {
		return nil, err
	}

	if serviceId != nil {
		result.ServiceName, err = getNameFromRedis("service", serviceId.String())
		if err != nil {
			return nil, err
		}
	}

	return &result, nil
}

// redisLookupResult defines the structure of the Redis message we're interested in.
type redisLookupResult struct {
	HostName    string `json:"-"` // Name of the host (never empty).
	ServiceName string `json:"-"` // Name of the service (only set in service context).
}
