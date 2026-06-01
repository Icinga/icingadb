package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/icinga/icinga-go-library/backoff"
	"github.com/icinga/icinga-go-library/com"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/retry"
	"github.com/icinga/icinga-go-library/types"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/theory/jsonpath"
	"github.com/theory/jsonpath/spec"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// panicForInvalidType panics if the request parameter is not in allowList. Used to safeguard internal queries.
func panicForInvalidType(allowList []string, request string) {
	if !slices.Contains(allowList, request) {
		panic("unexpected and unsupported object type \"" + request + "\"")
	}
}

// fetchObjectsFromRedis returns the objects with the requested ids of type typ from Redis.
func (client *Client) fetchObjectsFromRedis(
	ctx context.Context, typ string, ids ...types.Binary,
) (map[string]*icingaObject, error) {
	panicForInvalidType([]string{"host", "service"}, typ)

	if len(ids) == 0 {
		return nil, nil
	}

	key := "icinga:" + typ
	strIds := make([]string, 0, len(ids))
	for _, id := range ids {
		strIds = append(strIds, id.String())
	}

	var out map[string]*icingaObject
	err := retry.WithBackoff(
		ctx,
		func(ctx context.Context) error {
			out = make(map[string]*icingaObject)
			pairCh, errCh := client.redisClient.HMYield(ctx, key, strIds...)

			g, ctx := errgroup.WithContext(ctx)

			com.ErrgroupReceive(g, errCh)
			g.Go(func() error {
				for {
					select {
					case <-ctx.Done():
						return ctx.Err()

					case pair, ok := <-pairCh:
						if !ok {
							return nil
						}

						obj := new(icingaObject)
						if err := json.Unmarshal([]byte(pair.Value), obj); err != nil {
							return errors.Wrapf(err,
								"cannot JSON unmarshal Redis HMGET result for %q with key %q",
								key, pair.Field)
						}
						out[pair.Field] = obj
					}
				}
			})

			return g.Wait()
		},
		retry.Retryable,
		backoff.DefaultBackoff,
		retry.Settings{Timeout: retry.DefaultTimeout},
	)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot HMGET %q, %q from Redis", key, strIds)
	}

	return out, nil
}

// fetchObjectFromRedis returns the single object with the requested id of type typ from Redis.
func (client *Client) fetchObjectFromRedis(ctx context.Context, typ string, id types.Binary) (*icingaObject, error) {
	multi, err := client.fetchObjectsFromRedis(ctx, typ, id)
	if err != nil {
		return nil, err
	}

	obj, ok := multi[id.String()]
	if !ok {
		return nil, errors.Errorf("HMGET %q %q has an empty result", typ, id)
	}

	return obj, nil
}

// fetchObjectFromSql executes a query with the ids parameter and returns the output in the output argument pointer.
func (client *Client) fetchObjectFromSql(ctx context.Context, output any, query string, ids ...any) error {
	return retry.WithBackoff(
		ctx,
		func(ctx context.Context) error {
			query := client.db.Rebind(query)

			stmt, err := client.db.Preparex(query) //nolint:sqlclosecheck // deferred close below
			if err != nil {
				return err
			}
			defer func() { _ = stmt.Close() }()

			err = stmt.SelectContext(ctx, output, ids...)
			if err != nil {
				return database.CantPerformQuery(err, query)
			}

			return nil
		},
		retry.Retryable,
		backoff.DefaultBackoff,
		client.db.GetDefaultRetrySettings())
}

// fetchServiceIdsFromSql returns all service IDs for the given host ID from the SQL database.
func (client *Client) fetchServiceIdsFromSql(ctx context.Context, id types.Binary) ([]types.Binary, error) {
	var out []types.Binary

	err := client.fetchObjectFromSql(ctx, &out, `SELECT id FROM service WHERE host_id = ?`, id)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// fetchGroupsFromSql returns all groups of the requested typ for the parent id.
//
// If typ is "servicegroup" and hostIdForServices is set, the id will be interpreted as a host ID, resulting in all
// service groups for all host services.
func (client *Client) fetchGroupsFromSql(
	ctx context.Context, typ string, id types.Binary, hostIdForServices bool,
) ([]*icingaObject, error) {
	panicForInvalidType([]string{"hostgroup", "servicegroup"}, typ)

	var queryBuilder strings.Builder

	fmt.Fprintln(&queryBuilder, `SELECT `+typ+`.name AS name, `+typ+`.display_name AS display_name FROM `+typ)
	fmt.Fprintln(&queryBuilder, `JOIN `+typ+`_member ON `+typ+`.id = `+typ+`_member.`+typ+`_id`)

	if hostIdForServices {
		fmt.Fprintln(&queryBuilder, `JOIN service on servicegroup_member.service_id = service.id`)
		fmt.Fprintln(&queryBuilder, `WHERE service.host_id = ?`)
	} else {
		fmt.Fprintln(&queryBuilder, `WHERE `+typ+`_member.`+strings.TrimSuffix(typ, "group")+`_id = ?`)
	}

	fmt.Fprintln(&queryBuilder, `GROUP BY `+typ+`.name, `+typ+`.display_name`)

	var objs []*icingaObject
	err := client.fetchObjectFromSql(ctx, &objs, queryBuilder.String(), id)
	if err != nil {
		return nil, err
	}

	return objs, nil
}

// fetchMultiCustomVarsFromSql returns all custom variables for requested IDs of type typ from the relational database.
func (client *Client) fetchMultiCustomVarsFromSql(
	ctx context.Context, typ string, ids ...types.Binary,
) (map[string]map[string]any, error) {
	// While host and service groups could have customvars, this is currently explicitly not supported in Notifications.
	panicForInvalidType([]string{"host", "service"}, typ)

	if len(ids) == 0 {
		return nil, nil
	}

	type customVar struct {
		TypId types.Binary `db:"typ_id"`
		Name  string       `db:"name"`
		Value string       `db:"value"`
	}

	query, args, err := sqlx.In(
		`SELECT
			`+typ+`_customvar.`+typ+`_id AS typ_id,
			customvar.name AS name,
			customvar.value AS value
		FROM `+typ+`_customvar
		JOIN customvar
		ON `+typ+`_customvar.customvar_id = customvar.id
		WHERE `+typ+`_customvar.`+typ+`_id IN (?)`,
		ids)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create IN query")
	}

	var customVars []customVar
	err = client.fetchObjectFromSql(ctx, &customVars, query, args...)
	if err != nil {
		return nil, err
	}

	vars := make(map[string]map[string]any)
	for _, customVar := range customVars {
		var customVarValue any

		customVarMap, ok := vars[customVar.TypId.String()]
		if !ok {
			customVarMap = make(map[string]any)
			vars[customVar.TypId.String()] = customVarMap
		}

		if err := json.Unmarshal([]byte(customVar.Value), &customVarValue); err != nil {
			return nil, errors.Wrapf(err,
				"cannot unmarshal JSON value of custom var %q from %s object %q",
				customVar.Name, typ, customVar.TypId.String())
		}
		customVarMap[customVar.Name] = customVarValue
	}

	return vars, nil
}

// fetchCustomVarsFromSql returns all custom variables for the requested ID of type typ from the relational database.
func (client *Client) fetchCustomVarsFromSql(ctx context.Context, typ string, id types.Binary) (map[string]any, error) {
	multi, err := client.fetchMultiCustomVarsFromSql(ctx, typ, id)
	if err != nil {
		return nil, err
	}

	if len(multi) == 0 {
		// Objects without any custom vars are also valid.
		return nil, nil
	}

	return multi[id.String()], nil
}

// icingaObject represents an Icinga struct, such as a Host or Service.
type icingaObject struct {
	Name        string `json:"name" db:"name"`
	DisplayName string `json:"display_name" db:"display_name"`
	// Vars might be unpopulated for certain types, such as host or service groups.
	Vars map[string]any `json:"vars,omitempty" db:"-"`
}

// relations of an object to be passed to Icinga Notifications.
type relations struct {
	// hostId and serviceId, where the latter can be nil for hosts.
	hostId    types.Binary
	serviceId types.Binary

	// provider to complete this relations object.
	provider func(context.Context, *relations, []*spec.Segment) error

	// completeRelations lists JSONPaths of completed relations, to be populated by the provider.
	completeRelations []string

	// Following fields are exported to the event.Event.
	Object        struct{ Type string }
	Host          *icingaObject
	Services      []*icingaObject
	Hostgroups    []*icingaObject
	Servicegroups []*icingaObject
}

// populateObject fills the Object struct and completeRelations.
func (rel *relations) populateObject() {
	if rel.serviceId == nil {
		rel.Object.Type = "host"
	} else {
		rel.Object.Type = "service"
	}

	if !slices.Contains(rel.completeRelations, "object.type") {
		rel.completeRelations = append(rel.completeRelations, "object.type")
	}
}

// asMap creates a Go map populated by this relations and all its fields to be used in event.Event.
//
// Except the "object", all map values are one or multiple icingaObjects, which are JSON serializable.
func (rel *relations) asMap() map[string]any {
	out := make(map[string]any)

	out["object"] = map[string]any{"type": rel.Object.Type}

	if rel.Host != nil {
		out["host"] = rel.Host
	}
	if len(rel.Services) > 0 {
		out["services"] = rel.Services
	}
	if len(rel.Hostgroups) > 0 {
		out["hostgroups"] = rel.Hostgroups
	}
	if len(rel.Servicegroups) > 0 {
		out["servicegroups"] = rel.Servicegroups
	}

	return out
}

// complete the relations based on the query path.
func (rel *relations) complete(ctx context.Context, query string) error {
	path, err := jsonpath.Parse(query)
	if err != nil {
		return errors.Wrapf(err, "cannot parse JSONPath %q", query)
	}

	return rel.provider(ctx, rel, path.Query().Segments())
}

// relationsProvider implements a relations provider backed by the Client, to be replaced for testing.
func (client *Client) relationsProvider(ctx context.Context, rel *relations, segments []*spec.Segment) error {
	if len(segments) == 0 {
		return errors.New("cannot provide relations for empty query")
	}

	for _, selector := range segments[0].Selectors() {
		rootSelector := selector.String()
		childSegments := segments[1:]

		if rootSelector == `"object"` {
			// "object" is a special case and already completely populated.
			continue
		}

		var fetchObject, fetchCustomVars bool
		if len(childSegments) == 0 {
			fetchObject = true
			fetchCustomVars = true
		} else {
			// Host is the only directly used object, while every other object is within an array, one layer deeper.
			var selectors []spec.Selector
			if rootSelector == `"host"` {
				selectors = childSegments[0].Selectors()
			} else if len(childSegments) == 1 {
				// No field of an array object child is being referenced. We are dealing with something like
				// "services[*]" or "services.foo". However, as it is not supported to only fetch a certain service, all
				// services - or all instances of the object types - are being fetched. Due to the missing field, this
				// will be equivalent to "services[*][*]". So fetch everything and skip the for loop below.
				fetchObject = true
				fetchCustomVars = true
			} else {
				selectors = childSegments[1].Selectors()
			}

			for _, selector := range selectors {
				switch selector.String() {
				case `"vars"`:
					fetchCustomVars = true
				case `*`:
					fetchObject = true
					fetchCustomVars = true
				default:
					// For every other selector, only the object is being fetched. This might be "name", "display_name",
					// or another attribute of the object, hopefully existing. Please note, in the following "name" is
					// implicitly considered as the object attribute.
					fetchObject = true
				}
			}
		}

		updateCompleteRelations := func(prefix string, isGroup bool) {
			var newRelations []string

			if fetchObject {
				// Both name and display_name are fetched together from Redis.
				newRelations = append(newRelations, "name", "display_name")
			}
			if fetchCustomVars && !isGroup {
				newRelations = append(newRelations, "vars")
			}

			for _, newRelation := range newRelations {
				k := prefix + "." + newRelation
				if !slices.Contains(rel.completeRelations, k) {
					rel.completeRelations = append(rel.completeRelations, k)
				}
			}
		}

		switch rootSelector {
		case `"host"`:
			var hostObj *icingaObject
			var hostCustomVars map[string]any

			if slices.Contains(rel.completeRelations, "host.name") {
				fetchObject = false
				hostObj = rel.Host
			}
			if slices.Contains(rel.completeRelations, "host.vars") {
				fetchCustomVars = false
			}

			// Custom vars require the main object.
			if fetchCustomVars && !fetchObject && !slices.Contains(rel.completeRelations, "host.name") {
				fetchObject = true
			}

			if !fetchObject && !fetchCustomVars {
				continue
			}

			g, ctx := errgroup.WithContext(ctx)
			if fetchObject {
				g.Go(func() (err error) {
					hostObj, err = client.fetchObjectFromRedis(ctx, "host", rel.hostId)
					return
				})
			}
			if fetchCustomVars {
				g.Go(func() (err error) {
					hostCustomVars, err = client.fetchCustomVarsFromSql(ctx, "host", rel.hostId)
					return
				})
			}
			if err := g.Wait(); err != nil {
				return errors.Wrap(err, "cannot fetch host information")
			}

			hostObj.Vars = hostCustomVars

			updateCompleteRelations("host", false)
			rel.Host = hostObj
		case `"services"`:
			if rel.serviceId != nil {
				// For service objects, complete this service.
				var serviceObj *icingaObject
				var serviceCustomVars map[string]any

				if slices.Contains(rel.completeRelations, "services[*].name") {
					fetchObject = false
					serviceObj = rel.Services[0]
				}
				if slices.Contains(rel.completeRelations, "services[*].vars") {
					fetchCustomVars = false
				}

				// Custom vars require the main object.
				if fetchCustomVars && !fetchObject && !slices.Contains(rel.completeRelations, "services[*].name") {
					fetchObject = true
				}

				if !fetchObject && !fetchCustomVars {
					continue
				}

				g, ctx := errgroup.WithContext(ctx)
				if fetchObject {
					g.Go(func() (err error) {
						serviceObj, err = client.fetchObjectFromRedis(ctx, "service", rel.serviceId)
						return
					})
				}
				if fetchCustomVars {
					g.Go(func() (err error) {
						serviceCustomVars, err = client.fetchCustomVarsFromSql(ctx, "service", rel.serviceId)
						return
					})
				}
				if err := g.Wait(); err != nil {
					return errors.Wrap(err, "cannot fetch service information")
				}

				serviceObj.Vars = serviceCustomVars

				updateCompleteRelations("services[*]", false)
				rel.Services = []*icingaObject{serviceObj}
			} else {
				// For host objects, complete all services.
				if slices.Contains(rel.completeRelations, "services[*].name") && !fetchCustomVars {
					continue
				}
				if slices.Contains(rel.completeRelations, "services[*].vars") {
					continue
				}

				// As the JSON schema expects a list of service objects and not an object including the service IDs, the
				// IDs are re-fetched every time, as caching those IDs adds another layer of complexity.
				serviceIds, err := client.fetchServiceIdsFromSql(ctx, rel.hostId)
				if err != nil {
					return errors.Wrap(err, "cannot fetch host services")
				}

				if fetchCustomVars {
					fetchObject = true
				}

				var serviceObjs map[string]*icingaObject
				var serviceCustomVars map[string]map[string]any

				g, ctx := errgroup.WithContext(ctx)
				if fetchObject {
					g.Go(func() (err error) {
						serviceObjs, err = client.fetchObjectsFromRedis(ctx, "service", serviceIds...)
						return
					})
				}
				if fetchCustomVars {
					g.Go(func() (err error) {
						serviceCustomVars, err = client.fetchMultiCustomVarsFromSql(ctx, "service", serviceIds...)
						return
					})
				}
				if err := g.Wait(); err != nil {
					return errors.Wrap(err, "cannot fetch host service information")
				}

				services := make([]*icingaObject, 0, len(serviceObjs))
				for id, service := range serviceObjs {
					if len(serviceCustomVars) > 0 {
						service.Vars = serviceCustomVars[id]
					}

					services = append(services, service)
				}

				updateCompleteRelations("services[*]", false)
				rel.Services = services
			}
		case `"hostgroups"`:
			if slices.Contains(rel.completeRelations, "hostgroups[*].name") {
				continue
			}

			hostGroups, err := client.fetchGroupsFromSql(ctx, "hostgroup", rel.hostId, false)
			if err != nil {
				return errors.Wrap(err, "cannot fetch hostgroups")
			}

			updateCompleteRelations("hostgroups[*]", true)
			rel.Hostgroups = hostGroups
		case `"servicegroups"`:
			if slices.Contains(rel.completeRelations, "servicegroups[*].name") {
				continue
			}

			var serviceGroups []*icingaObject
			var err error

			if rel.serviceId != nil {
				serviceGroups, err = client.fetchGroupsFromSql(ctx, "servicegroup", rel.serviceId, false)
			} else {
				serviceGroups, err = client.fetchGroupsFromSql(ctx, "servicegroup", rel.hostId, true)
			}
			if err != nil {
				return errors.Wrap(err, "cannot fetch servicegroups")
			}

			updateCompleteRelations("servicegroups[*]", true)
			rel.Servicegroups = serviceGroups
		default:
			return errors.Errorf("unsupported JSONPath root segment selector %q", rootSelector)
		}

		client.logger.Debugw("Client evaluated relation selector",
			zap.String("root_selector", rootSelector),
			zap.Stringers("child_segments", childSegments),
			zap.Strings("complete_relations", rel.completeRelations))
	}

	return nil
}

// fetchHostServiceData resolves the object names and fetches the associated custom variables.
//
// If serviceId is not nil, both host and service data will be queried. Otherwise, only host information is fetched.
// Later, the relations can get extended via relations.complete.
func (client *Client) fetchHostServiceData(
	ctx context.Context,
	hostId, serviceId types.Binary,
) (*relations, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rel := &relations{
		hostId:    hostId,
		serviceId: serviceId,

		provider: client.relationsProvider,
	}
	rel.populateObject()

	if err := rel.complete(ctx, "$.host.name"); err != nil {
		return nil, err
	}
	if serviceId != nil {
		if err := rel.complete(ctx, "$.services[*].name"); err != nil {
			return nil, err
		}
	}

	return rel, nil
}
