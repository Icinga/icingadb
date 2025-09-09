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
	redisHGet := func(typ, field string, out *redisLookupResult) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		err := retry.WithBackoff(
			ctx,
			func(ctx context.Context) error { return client.redisClient.HGet(ctx, "icinga:"+typ, field).Scan(out) },
			retry.Retryable,
			backoff.DefaultBackoff,
			retry.Settings{},
		)
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return fmt.Errorf("%s with ID %s not found in Redis", typ, hostId)
			}
			return fmt.Errorf("failed to fetch %s with ID %s from Redis: %w", typ, field, err)
		}
		return nil
	}

	var result redisLookupResult
	if err := redisHGet("host", hostId.String(), &result); err != nil {
		return nil, err
	}

	result.HostName = result.Name
	result.Name = "" // Clear the name field for the host, as we will fetch the service name next.

	if serviceId != nil {
		if err := redisHGet("service", serviceId.String(), &result); err != nil {
			return nil, err
		}
		result.ServiceName = result.Name
		result.Name = "" // It's not needed anymore, clear it!
	}

	return &result, nil
}

// redisLookupResult defines the structure of the Redis message we're interested in.
type redisLookupResult struct {
	HostName    string `json:"-"` // Name of the host (never empty).
	ServiceName string `json:"-"` // Name of the service (only set in service context).

	// Name is used to retrieve the host or service name from Redis.
	// It should not be used for any other purpose apart from within the [Client.fetchHostServiceName] function.
	Name string `json:"name"`
}

// UnmarshalBinary implements the [encoding.BinaryUnmarshaler] interface for redisLookupResult.
//
// It unmarshals the binary data of the Redis HGet result into the redisLookupResult struct.
// This is required for the HGet().Scan() usage in the [Client.fetchHostServiceName] function to work correctly.
func (rlr *redisLookupResult) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return errors.New("empty data received for redisLookupResult")
	}

	if err := json.Unmarshal(data, rlr); err != nil {
		return fmt.Errorf("failed to unmarshal redis result: %w", err)
	}
	return nil
}
