package notifications

import (
	"context"
	"encoding/json"
	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"time"
)

// fetchHostServiceFromRedis retrieves the host and service names from Redis.
//
// If serviceId is nil, only the host name is fetched. Otherwise, both host and service name is fetched.
func (client *Client) fetchHostServiceFromRedis(
	ctx context.Context,
	hostId, serviceId types.Binary,
) (hostName string, serviceName string, err error) {
	getNameFromRedis := func(ctx context.Context, typ, id string) (string, error) {
		key := "icinga:" + typ

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
			return "", errors.Wrapf(err, "redis HGET %q, %q failed", key, id)
		}

		var result struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			return "", errors.Wrap(err, "failed to unmarshal redis result")
		}

		return result.Name, nil
	}

	hostName, err = getNameFromRedis(ctx, "host", hostId.String())
	if err != nil {
		return
	}

	if serviceId != nil {
		serviceName, err = getNameFromRedis(ctx, "service", serviceId.String())
		if err != nil {
			return
		}
	}

	return
}

// customVar is used as an internal representation in Client.fetchCustomVarFromSql.
type customVar struct {
	Name  string       `db:"name"`
	Value types.String `db:"value"`
}

// getValue returns this customvar's value as a string, transforming SQL NULLs to empty strings.
func (cv customVar) getValue() string {
	if cv.Value.Valid {
		return cv.Value.String
	}
	return ""
}

// fetchCustomVarFromSql retrieves custom variables for the host and service from SQL.
//
// If serviceId is nil, only the host custom vars are fetched. Otherwise, both host and service custom vars are fetched.
func (client *Client) fetchCustomVarFromSql(
	ctx context.Context,
	hostId, serviceId types.Binary,
) (map[string]string, error) {
	getCustomVarsFromSql := func(ctx context.Context, typ string, id types.Binary) ([]customVar, error) {
		stmt, err := client.db.Preparex(client.db.Rebind(
			`SELECT
				customvar_flat.flatname AS name,
				customvar_flat.flatvalue AS value
			FROM ` + typ + `_customvar
			JOIN customvar_flat
			ON ` + typ + `_customvar.customvar_id = customvar_flat.customvar_id
			WHERE ` + typ + `_customvar.` + typ + `_id = ?`))
		if err != nil {
			return nil, err
		}
		defer func() { _ = stmt.Close() }()

		var customVars []customVar
		if err := stmt.SelectContext(ctx, &customVars, id); err != nil {
			return nil, err
		}

		return customVars, nil
	}

	customVars := make(map[string]string)

	hostVars, err := getCustomVarsFromSql(ctx, "host", hostId)
	if err != nil {
		return nil, err
	}

	for _, hostVar := range hostVars {
		customVars["host.vars."+hostVar.Name] = hostVar.getValue()
	}

	if serviceId != nil {
		serviceVars, err := getCustomVarsFromSql(ctx, "service", serviceId)
		if err != nil {
			return nil, err
		}

		for _, serviceVar := range serviceVars {
			customVars["service.vars."+serviceVar.Name] = serviceVar.getValue()
		}
	}

	return customVars, nil
}

// hostServiceInformation contains the host name, an optional service name, and all custom variables.
//
// Returned from Client.fetchHostServiceData.
type hostServiceInformation struct {
	hostName    string
	serviceName string
	customVars  map[string]string
}

// fetchHostServiceData resolves the object names and fetches the associated custom variables.
//
// If serviceId is not nil, both host and service data will be queried. Otherwise, only host information is fetched. To
// acquire the information, the fetchHostServiceFromRedis and fetchCustomVarFromSql methods are used concurrently with
// a timeout of three seconds.
func (client *Client) fetchHostServiceData(
	ctx context.Context,
	hostId, serviceId types.Binary,
) (*hostServiceInformation, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	ret := &hostServiceInformation{}
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		ret.hostName, ret.serviceName, err = client.fetchHostServiceFromRedis(ctx, hostId, serviceId)
		return err
	})
	g.Go(func() error {
		var err error
		ret.customVars, err = client.fetchCustomVarFromSql(ctx, hostId, serviceId)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return ret, nil
}
