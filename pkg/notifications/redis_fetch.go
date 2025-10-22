package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
)

// redisCustomVar is a customvar entry from Redis.
type redisCustomVar struct {
	EnvironmentID types.Binary `json:"environment_id"`
	Name          string       `json:"name"`
	Value         string       `json:"value"`
}

// redisLookupResult defines the structure of the Redis message we're interested in.
type redisLookupResult struct {
	hostName    string
	serviceName string
	customVars  []*redisCustomVar
}

// CustomVars returns a mapping of customvar names to values.
func (result redisLookupResult) CustomVars() map[string]string {
	m := make(map[string]string)
	for _, customvar := range result.customVars {
		m[customvar.Name] = customvar.Value
	}

	return m
}

// fetchHostServiceFromRedis retrieves the host and service names and customvars from Redis.
//
// It uses either the hostId or/and serviceId to fetch the corresponding names. If both are provided,
// the returned result will contain the host name and the service name accordingly. Otherwise, it will
// only contain the host name.
//
// The function has a hard coded timeout of five seconds for all HGET and HGETALL commands together.
func (client *Client) fetchHostServiceFromRedis(ctx context.Context, hostId, serviceId types.Binary) (*redisLookupResult, error) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	hgetFromRedis := func(key, id string) (string, error) {
		var data string
		err := retry.WithBackoff(
			ctx,
			func(ctx context.Context) (err error) {
				data, err = client.redisClient.HGet(ctx, key, id).Result()
				return
			},
			retry.Retryable,
			backoff.DefaultBackoff,
			retry.Settings{},
		)
		if err != nil {
			return "", fmt.Errorf("redis hget %q, %q failed: %w", key, id, err)
		}

		return data, nil
	}

	getNameFromRedis := func(typ, id string) (string, error) {
		data, err := hgetFromRedis("icinga:"+typ, id)
		if err != nil {
			return "", err
		}

		var result struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			return "", fmt.Errorf("failed to unmarshal redis result: %w", err)
		}

		return result.Name, nil
	}

	getCustomVarFromRedis := func(id string) (*redisCustomVar, error) {
		data, err := hgetFromRedis("icinga:customvar", id)
		if err != nil {
			return nil, err
		}

		customvar := new(redisCustomVar)
		if err := json.Unmarshal([]byte(data), customvar); err != nil {
			return nil, fmt.Errorf("failed to unmarshal redis result: %w", err)
		}

		return customvar, nil
	}

	getObjectCustomVarsFromRedis := func(typ, id string) ([]*redisCustomVar, error) {
		var resMap map[string]string
		err := retry.WithBackoff(
			ctx,
			func(ctx context.Context) (err error) {
				res := client.redisClient.HGetAll(ctx, "icinga:"+typ+":customvar")
				if err = res.Err(); err != nil {
					return
				}

				resMap, err = res.Result()
				return
			},
			retry.Retryable,
			backoff.DefaultBackoff,
			retry.Settings{},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to HGETALL icinga:%s:customvar from Redis: %w", typ, err)
		}

		var result struct {
			CustomvarId string `json:"customvar_id"`
			HostId      string `json:"host_id"`
			ServiceId   string `json:"service_id"`
		}

		var customvars []*redisCustomVar
		for _, res := range resMap {
			if err := json.Unmarshal([]byte(res), &result); err != nil {
				return nil, fmt.Errorf("failed to unmarshal redis result: %w", err)
			}

			switch typ {
			case "host":
				if result.HostId != id {
					continue
				}
			case "service":
				if result.ServiceId != id {
					continue
				}
			default:
				panic(fmt.Sprintf("unexpected object type %q", typ))
			}

			customvar, err := getCustomVarFromRedis(result.CustomvarId)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch customvar: %w", err)
			}
			customvars = append(customvars, customvar)
		}

		return customvars, nil
	}

	var result redisLookupResult
	var err error

	result.hostName, err = getNameFromRedis("host", hostId.String())
	if err != nil {
		return nil, err
	}

	if serviceId != nil {
		result.serviceName, err = getNameFromRedis("service", serviceId.String())
		if err != nil {
			return nil, err
		}
	}

	if serviceId == nil {
		result.customVars, err = getObjectCustomVarsFromRedis("host", hostId.String())
	} else {
		result.customVars, err = getObjectCustomVarsFromRedis("service", serviceId.String())
	}
	if err != nil {
		return nil, err
	}

	return &result, nil
}
