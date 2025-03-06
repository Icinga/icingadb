package icingadb_test

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-testing/services"
	"github.com/icinga/icinga-testing/utils"
	"github.com/icinga/icinga-testing/utils/eventually"
	localutils "github.com/icinga/icingadb/tests/internal/utils"
	"github.com/icinga/icingadb/tests/internal/value"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	"io"
	"reflect"
	"sort"
	"strings"
	"testing"
	"text/template"
	"time"
)

//go:embed object_sync_test.conf
var testSyncConfRaw string
var testSyncConfTemplate = template.Must(template.New("testdata.conf").Funcs(template.FuncMap{"NaturalJoin": strings.Join}).Parse(testSyncConfRaw))

var usergroups = []string{
	"testusergroup1",
	"testusergroup2",
	"testusergroup3",
}

// Map of users to a set of their groups
var users = map[string]map[string]struct{}{
	"testuser1": {"testusergroup1": {}, "testusergroup3": {}},
	"testuser2": {"testusergroup2": {}},
	"testuser3": {"testusergroup3": {}, "testusergroup1": {}},
}

func TestObjectSync(t *testing.T) {
	logger := it.Logger(t)

	type Data struct {
		GenericPrefixes        []string
		Hosts                  []Host
		Services               []Service
		Users                  []User
		Notifications          []Notification
		NotificationUsers      map[string]map[string]struct{}
		NotificationUserGroups []string
		DependencyGroups       []DependencyGroup
	}
	data := &Data{
		// Some name prefixes to loop over in the template to generate multiple instances of objects,
		// for example default-host, some-host, and other-host.
		GenericPrefixes: []string{"default", "some", "other"},

		Hosts:                  makeTestSyncHosts(t),
		Services:               makeTestSyncServices(t),
		Users:                  makeTestUsers(t),
		Notifications:          makeTestNotifications(t),
		NotificationUsers:      users,
		NotificationUserGroups: usergroups,
		DependencyGroups:       makeTestDependencyGroups(),
	}

	r := it.RedisServerT(t)
	rdb := getDatabase(t)
	i := it.Icinga2NodeT(t, "master")
	conf := bytes.NewBuffer(nil)
	err := testSyncConfTemplate.Execute(conf, data)
	require.NoError(t, err, "render icinga2 config")
	for _, host := range data.Hosts {
		err := writeIcinga2ConfigObject(conf, host)
		require.NoError(t, err, "generate icinga2 host config")
	}
	for _, service := range data.Services {
		err := writeIcinga2ConfigObject(conf, service)
		require.NoError(t, err, "generate icinga2 service config")
	}
	for _, user := range data.Users {
		err := writeIcinga2ConfigObject(conf, user)
		require.NoError(t, err, "generate icinga2 user config")
	}
	for _, notification := range data.Notifications {
		err := writeIcinga2ConfigObject(conf, notification)
		require.NoError(t, err, "generate icinga2 notification config")
	}
	//logger.Sugar().Infof("config:\n\n%s\n\n", conf.String())
	i.WriteConfig("etc/icinga2/conf.d/testdata.conf", conf.Bytes())
	i.EnableIcingaDb(r)
	require.NoError(t, i.Reload(), "reload Icinga 2 daemon")

	// Wait for Icinga 2 to signal a successful dump before starting
	// Icinga DB to ensure that we actually test the initial sync.
	logger.Debug("waiting for icinga2 dump done signal")
	waitForDumpDoneSignal(t, r, 20*time.Second, 100*time.Millisecond)

	// Only after that, start Icinga DB.
	logger.Debug("starting icingadb")
	it.IcingaDbInstanceT(t, r, rdb)

	db, err := sqlx.Open(rdb.Driver(), rdb.DSN())
	require.NoError(t, err, "connecting to SQL database shouldn't fail")
	t.Cleanup(func() { _ = db.Close() })

	t.Run("Host", func(t *testing.T) {
		t.Parallel()

		for _, host := range data.Hosts {
			host := host
			t.Run("Verify-"+host.VariantInfoString(), func(t *testing.T) {
				t.Parallel()

				eventually.Assert(t, func(t require.TestingT) {
					verifyIcingaDbRow(t, db, host)
				}, 20*time.Second, 1*time.Second)

				if host.Vars != nil {
					t.Run("CustomVar", func(t *testing.T) {
						logger := it.Logger(t)
						eventually.Assert(t, func(t require.TestingT) {
							host.Vars.VerifyCustomVar(t, logger, db, host)
						}, 20*time.Second, 1*time.Second)
					})

					t.Run("CustomVarFlat", func(t *testing.T) {
						logger := it.Logger(t)
						eventually.Assert(t, func(t require.TestingT) {
							host.Vars.VerifyCustomVarFlat(t, logger, db, host)
						}, 20*time.Second, 1*time.Second)
					})
				}
			})

		}
	})

	t.Run("Service", func(t *testing.T) {
		t.Parallel()

		for _, service := range data.Services {
			service := service
			t.Run("Verify-"+service.VariantInfoString(), func(t *testing.T) {
				t.Parallel()

				eventually.Assert(t, func(t require.TestingT) {
					verifyIcingaDbRow(t, db, service)
				}, 20*time.Second, 1*time.Second)

				if service.Vars != nil {
					t.Run("CustomVar", func(t *testing.T) {
						logger := it.Logger(t)
						eventually.Assert(t, func(t require.TestingT) {
							service.Vars.VerifyCustomVar(t, logger, db, service)
						}, 20*time.Second, 1*time.Second)
					})

					t.Run("CustomVarFlat", func(t *testing.T) {
						logger := it.Logger(t)
						eventually.Assert(t, func(t require.TestingT) {
							service.Vars.VerifyCustomVarFlat(t, logger, db, service)
						}, 20*time.Second, 1*time.Second)
					})
				}
			})
		}
	})

	t.Run("HostGroup", func(t *testing.T) {
		t.Parallel()
		// TODO(jb): add tests

		t.Run("Member", func(t *testing.T) { t.Parallel(); t.Skip() })
		t.Run("CustomVar", func(t *testing.T) { t.Parallel(); t.Skip() })
	})

	t.Run("ServiceGroup", func(t *testing.T) {
		t.Parallel()
		// TODO(jb): add tests

		t.Run("Member", func(t *testing.T) { t.Parallel(); t.Skip() })
		t.Run("CustomVar", func(t *testing.T) { t.Parallel(); t.Skip() })
	})

	t.Run("Endpoint", func(t *testing.T) {
		t.Parallel()
		// TODO(jb): add tests

		t.Skip()
	})

	for _, commandType := range []string{"CheckCommend", "EventCommand", "NotificationCommand"} {
		commandType := commandType
		t.Run(commandType, func(t *testing.T) {
			t.Parallel()
			// TODO(jb): add tests

			t.Run("Argument", func(t *testing.T) { t.Parallel(); t.Skip() })
			t.Run("EnvVar", func(t *testing.T) { t.Parallel(); t.Skip() })
			t.Run("CustomVar", func(t *testing.T) { t.Parallel(); t.Skip() })
		})
	}

	t.Run("Comment", func(t *testing.T) {
		t.Parallel()
		// TODO(jb): add tests

		t.Skip()
	})

	t.Run("Downtime", func(t *testing.T) {
		t.Parallel()
		// TODO(jb): add tests

		t.Skip()
	})

	t.Run("Notification", func(t *testing.T) {
		t.Parallel()
		for _, notification := range data.Notifications {
			notification := notification
			t.Run("Verify-"+notification.VariantInfoString(), func(t *testing.T) {
				t.Parallel()

				eventually.Assert(t, func(t require.TestingT) {
					notification.verify(t, db)
				}, 20*time.Second, 1*time.Second)

				if notification.Vars != nil {
					t.Run("CustomVar", func(t *testing.T) {
						logger := it.Logger(t)
						eventually.Assert(t, func(t require.TestingT) {
							notification.Vars.VerifyCustomVar(t, logger, db, notification)
						}, 20*time.Second, 1*time.Second)
					})

					t.Run("CustomVarFlat", func(t *testing.T) {
						logger := it.Logger(t)
						eventually.Assert(t, func(t require.TestingT) {
							notification.Vars.VerifyCustomVarFlat(t, logger, db, notification)
						}, 20*time.Second, 1*time.Second)
					})
				}
			})
		}
	})

	t.Run("TimePeriod", func(t *testing.T) {
		t.Parallel()
		// TODO(jb): add tests

		t.Run("Range", func(t *testing.T) { t.Parallel(); t.Skip() })
		t.Run("OverrideInclude", func(t *testing.T) { t.Parallel(); t.Skip() })
		t.Run("OverrideExclude", func(t *testing.T) { t.Parallel(); t.Skip() })
		t.Run("CustomVar", func(t *testing.T) { t.Parallel(); t.Skip() })
	})

	t.Run("CustomVar", func(t *testing.T) {
		t.Parallel()
		// TODO(jb): add tests

		t.Skip()
	})

	t.Run("CustomVarFlat", func(t *testing.T) {
		t.Parallel()
		// TODO(jb): add tests

		t.Skip()
	})

	t.Run("User", func(t *testing.T) {
		t.Parallel()

		for _, user := range data.Users {
			user := user
			t.Run("Verify-"+user.VariantInfoString(), func(t *testing.T) {
				t.Parallel()

				eventually.Assert(t, func(t require.TestingT) {
					verifyIcingaDbRow(t, db, user)
				}, 20*time.Second, 1*time.Second)
			})
		}

		t.Run("UserCustomVar", func(t *testing.T) {
			t.Parallel()
			// TODO(jb): add tests

			t.Skip()
		})
	})

	t.Run("UserGroup", func(t *testing.T) {
		t.Parallel()
		// TODO(jb): add tests

		t.Run("Member", func(t *testing.T) { t.Parallel(); t.Skip() })
		t.Run("CustomVar", func(t *testing.T) { t.Parallel(); t.Skip() })
	})

	t.Run("Zone", func(t *testing.T) {
		t.Parallel()
		// TODO(jb): add tests

		t.Skip()
	})

	t.Run("Dependencies", func(t *testing.T) {
		t.Parallel()

		t.Cleanup(func() { assertNoDependencyDanglingReferences(t, r, db) })

		for _, dependencyGroup := range data.DependencyGroups {
			t.Run("Verify-"+dependencyGroup.InfoString(), func(t *testing.T) {
				t.Parallel()

				eventually.Assert(t, func(t require.TestingT) {
					dependencyGroup.verify(t, db)
				}, 20*time.Second, 200*time.Millisecond)
			})
		}
	})

	t.Run("RuntimeUpdates", func(t *testing.T) {
		t.Parallel()

		// Wait some time to give Icinga DB a chance to finish the initial sync.
		// TODO(jb): properly wait for this? but I don't know of a good way to detect this at the moment
		time.Sleep(20 * time.Second)

		client := i.ApiClient()

		t.Run("Service", func(t *testing.T) {
			t.Parallel()

			for _, service := range makeTestSyncServices(t) {
				service := service

				t.Run("CreateAndDelete-"+service.VariantInfoString(), func(t *testing.T) {
					t.Parallel()

					client.CreateObject(t, "services", *service.HostName+"!"+service.Name, map[string]interface{}{
						"attrs": makeIcinga2ApiAttributes(service, false),
					})

					eventually.Assert(t, func(t require.TestingT) {
						verifyIcingaDbRow(t, db, service)
					}, 20*time.Second, 1*time.Second)

					if service.Vars != nil {
						t.Run("CustomVar", func(t *testing.T) {
							logger := it.Logger(t)
							eventually.Assert(t, func(t require.TestingT) {
								service.Vars.VerifyCustomVar(t, logger, db, service)
							}, 20*time.Second, 1*time.Second)
						})

						t.Run("CustomVarFlat", func(t *testing.T) {
							logger := it.Logger(t)
							eventually.Assert(t, func(t require.TestingT) {
								service.Vars.VerifyCustomVarFlat(t, logger, db, service)
							}, 20*time.Second, 1*time.Second)
						})
					}

					client.DeleteObject(t, "services", *service.HostName+"!"+service.Name, false)

					require.Eventuallyf(t, func() bool {
						var count int
						err := db.Get(&count, db.Rebind("SELECT COUNT(*) FROM service WHERE name = ?"), service.Name)
						require.NoError(t, err, "querying service count should not fail")
						return count == 0
					}, 20*time.Second, 1*time.Second, "service with name=%q should be removed from database", service.Name)
				})
			}

			t.Run("Update", func(t *testing.T) {
				t.Parallel()

				for _, service := range makeTestSyncServices(t) {
					service := service

					t.Run(service.VariantInfoString(), func(t *testing.T) {
						t.Parallel()

						// Start with the final host_name and zone. Finding out what happens when you change this on an
						// existing object might be fun, but not at this time.
						client.CreateObject(t, "services", *service.HostName+"!"+service.Name, map[string]interface{}{
							"attrs": map[string]interface{}{
								"check_command": "default-checkcommand",
								"zone":          service.ZoneName,
							},
						})
						require.Eventuallyf(t, func() bool {
							var count int
							err := db.Get(&count, db.Rebind("SELECT COUNT(*) FROM service WHERE name = ?"), service.Name)
							require.NoError(t, err, "querying service count should not fail")
							return count == 1
						}, 20*time.Second, 1*time.Second, "service with name=%q should exist in database", service.Name)

						client.UpdateObject(t, "services", *service.HostName+"!"+service.Name, map[string]interface{}{
							"attrs": makeIcinga2ApiAttributes(service, true),
						})

						eventually.Assert(t, func(t require.TestingT) {
							verifyIcingaDbRow(t, db, service)
						}, 20*time.Second, 1*time.Second)

						if service.Vars != nil {
							t.Run("CustomVar", func(t *testing.T) {
								logger := it.Logger(t)
								eventually.Assert(t, func(t require.TestingT) {
									service.Vars.VerifyCustomVar(t, logger, db, service)
								}, 20*time.Second, 1*time.Second)
							})

							t.Run("CustomVarFlat", func(t *testing.T) {
								logger := it.Logger(t)
								eventually.Assert(t, func(t require.TestingT) {
									service.Vars.VerifyCustomVarFlat(t, logger, db, service)
								}, 20*time.Second, 1*time.Second)
							})
						}

						client.DeleteObject(t, "services", *service.HostName+"!"+service.Name, false)
					})
				}
			})
		})

		t.Run("User", func(t *testing.T) {
			t.Parallel()

			for _, user := range makeTestUsers(t) {
				user := user

				t.Run("CreateAndDelete-"+user.VariantInfoString(), func(t *testing.T) {
					t.Parallel()

					client.CreateObject(t, "users", user.Name, map[string]interface{}{
						"attrs": makeIcinga2ApiAttributes(user, false),
					})

					eventually.Assert(t, func(t require.TestingT) {
						verifyIcingaDbRow(t, db, user)
					}, 20*time.Second, 1*time.Second)

					client.DeleteObject(t, "users", user.Name, false)

					require.Eventuallyf(t, func() bool {
						var count int
						err := db.Get(&count, db.Rebind(`SELECT COUNT(*) FROM "user" WHERE name = ?`), user.Name)
						require.NoError(t, err, "querying user count should not fail")
						return count == 0
					}, 20*time.Second, 1*time.Second, "user with name=%q should be removed from database", user.Name)
				})
			}

			t.Run("Update", func(t *testing.T) {
				t.Parallel()

				userName := utils.UniqueName(t, "user")

				client.CreateObject(t, "users", userName, map[string]interface{}{
					"attrs": map[string]interface{}{},
				})
				require.Eventuallyf(t, func() bool {
					var count int
					err := db.Get(&count, db.Rebind(`SELECT COUNT(*) FROM "user" WHERE name = ?`), userName)
					require.NoError(t, err, "querying user count should not fail")
					return count == 1
				}, 20*time.Second, 1*time.Second, "user with name=%q should exist in database", userName)

				for _, user := range makeTestUsers(t) {
					user := user
					user.Name = userName

					t.Run(user.VariantInfoString(), func(t *testing.T) {
						client.UpdateObject(t, "users", userName, map[string]interface{}{
							"attrs": makeIcinga2ApiAttributes(user, true),
						})

						eventually.Assert(t, func(t require.TestingT) {
							verifyIcingaDbRow(t, db, user)
						}, 20*time.Second, 1*time.Second)
					})
				}

				client.DeleteObject(t, "users", userName, false)
			})
		})

		t.Run("Notifications", func(t *testing.T) {
			t.Parallel()

			for _, notification := range makeTestNotifications(t) {
				notification := notification

				t.Run("CreateAndDelete-"+notification.VariantInfoString(), func(t *testing.T) {
					t.Parallel()

					client.CreateObject(t, "notifications", notification.fullName(), map[string]interface{}{
						"attrs": makeIcinga2ApiAttributes(notification, false),
					})

					eventually.Assert(t, func(t require.TestingT) {
						notification.verify(t, db)
					}, 20*time.Second, 200*time.Millisecond)

					client.DeleteObject(t, "notifications", notification.fullName(), false)

					require.Eventuallyf(t, func() bool {
						var count int
						err := db.Get(&count, db.Rebind("SELECT COUNT(*) FROM notification WHERE name = ?"), notification.fullName())
						require.NoError(t, err, "querying notification count should not fail")
						return count == 0
					}, 20*time.Second, 200*time.Millisecond, "notification with name=%q should be removed from database", notification.fullName())
				})
			}

			t.Run("Update", func(t *testing.T) {
				t.Parallel()

				baseNotification := Notification{
					Name:        utils.UniqueName(t, "notification"),
					HostName:    newString("default-host"),
					ServiceName: newString("default-service"),
					Command:     "default-notificationcommand",
					Users:       []string{"default-user"},
					UserGroups:  []string{"default-usergroup"},
					Interval:    1800,
				}

				client.CreateObject(t, "notifications", baseNotification.fullName(), map[string]interface{}{
					"attrs": makeIcinga2ApiAttributes(baseNotification, false),
				})

				require.Eventuallyf(t, func() bool {
					var count int
					err := db.Get(&count, db.Rebind("SELECT COUNT(*) FROM notification WHERE name = ?"), baseNotification.fullName())
					require.NoError(t, err, "querying notification count should not fail")
					return count == 1
				}, 20*time.Second, 200*time.Millisecond, "notification with name=%q should exist in database", baseNotification.fullName())

				// TODO: Currently broken, but has been tested manually multiple times. Gets more time after RC2
				/*t.Run("CreateAndDeleteUser", func(t *testing.T) {
					groupName := utils.UniqueName(t, "group")
					userName := "testuser112312321"

					// Create usergroup
					client.CreateObject(t, "usergroups", groupName, nil)

					baseNotification.UserGroups = []string{groupName}
					client.UpdateObject(t, "notifications", baseNotification.fullName(), map[string]interface{}{
						"attrs": map[string]interface{}{
							"user_groups": baseNotification.UserGroups,
						},
					})

					eventually.Assert(t, func(t require.TestingT) {
						baseNotification.verify(t, db)
					}, 20*time.Second, 1*time.Second)

					// Create user
					users[userName] = map[string]struct{}{groupName: {}}
					client.CreateObject(t, "users", userName, map[string]interface{}{
						"attrs": map[string]interface{}{
							"groups": baseNotification.UserGroups,
						},
					})

					require.Eventuallyf(t, func() bool {
						var count int
						err := db.Get(&count, "SELECT COUNT(*) FROM user WHERE name = ?", userName)
						require.NoError(t, err, "querying user count should not fail")
						return count == 1
					}, 20*time.Second, 200*time.Millisecond, "user with name=%q should exist in database", userName)

					eventually.Assert(t, func(t require.TestingT) {
						baseNotification.verify(t, db)
					}, 20*time.Second, 1*time.Second)

					// Delete user
					delete(users, userName)
					client.DeleteObject(t, "users", userName, false)

					eventually.Assert(t, func(t require.TestingT) {
						baseNotification.verify(t, db)
					}, 20*time.Second, 1*time.Second)

					// Remove group
					baseNotification.UserGroups = []string{}
					client.UpdateObject(t, "notifications", baseNotification.fullName(), map[string]interface{}{
						"attrs": map[string]interface{}{
							"user_groups": baseNotification.UserGroups,
						},
					})

					client.DeleteObject(t, "usergroups", groupName, false)

					eventually.Assert(t, func(t require.TestingT) {
						baseNotification.verify(t, db)
					}, 20*time.Second, 1*time.Second)
				})*/

				for _, notification := range makeTestNotifications(t) {
					notification := notification
					notification.Name = baseNotification.Name

					t.Run(notification.VariantInfoString(), func(t *testing.T) {
						client.UpdateObject(t, "notifications", notification.fullName(), map[string]interface{}{
							"attrs": makeIcinga2ApiAttributes(notification, true),
						})

						eventually.Assert(t, func(t require.TestingT) {
							notification.verify(t, db)
						}, 20*time.Second, 200*time.Millisecond)
					})
				}

				client.DeleteObject(t, "notifications", baseNotification.fullName(), false)
			})
		})

		t.Run("Dependencies", func(t *testing.T) {
			t.Parallel()

			// Make sure to check for any dangling references after all the subtests have run, i.e. as part of the
			// parent test (Dependencies) teardown process. Note, this isn't the same as using plain defer ..., as
			// all the subtests runs in parallel, and we want to make sure that the check is performed after all of
			// them have completed and not when this closure returns.
			t.Cleanup(func() { assertNoDependencyDanglingReferences(t, r, db) })

			for _, dependencyGroup := range makeTestDependencyGroups() {
				if !dependencyGroup.SendRuntimeUpdates {
					// This is probably something that should be tested only via the initial sync and not via runtime
					// updates, without having to build a bunch of special cases here. Instead, we can transform that
					// group into a completely different one and use it to simulate runtime created redundancy groups.
					dependencyGroup.Name = utils.RandomString(10)
					dependencyGroup.SkipVerification = false
					// We only need to drop the existing children, as we will add new ones down below and make
					// use of the existing parents.
					dependencyGroup.Children = []string{}
				}

				t.Run("CreateAndDelete-"+dependencyGroup.InfoString(), func(t *testing.T) {
					t.Parallel()

					newChildrenLen := 4
					if dependencyGroup.SendRuntimeUpdates {
						newChildrenLen = len(dependencyGroup.Children)
					}

					var newChildren []string
					for i := 0; i < newChildrenLen; i++ {
						child := utils.RandomString(10)
						newChildren = append(newChildren, child)
						client.CreateObject(t, "hosts", child, map[string]interface{}{
							"templates": []string{"dependency-host-template"},
						})

						for _, parent := range dependencyGroup.Parents {
							client.CreateObject(t, "dependencies", child+"!"+utils.RandomString(10), map[string]interface{}{
								"attrs": map[string]interface{}{
									"parent_host_name":   parent,
									"child_host_name":    child,
									"redundancy_group":   dependencyGroup.Name,
									"ignore_soft_states": dependencyGroup.IgnoreSoftStates,
									"period":             dependencyGroup.TimePeriod,
									"states":             dependencyGroup.StatesFilter,
								},
							})
						}
					}
					oldChildren := dependencyGroup.Children
					dependencyGroup.Children = append(oldChildren, newChildren...)

					eventually.Assert(t, func(t require.TestingT) {
						dependencyGroup.verify(t, db)
					}, 20*time.Second, 200*time.Millisecond)

					for _, child := range newChildren {
						client.DeleteObject(t, "hosts", child, true)
						dependencyGroup.Children = slices.DeleteFunc(dependencyGroup.Children, func(c string) bool {
							return c == child
						})

						eventually.Assert(t, func(t require.TestingT) {
							dependencyGroup.verify(t, db)
						}, 20*time.Second, 200*time.Millisecond)
					}

					// Restore the original children and perform a final verification with its original state.
					dependencyGroup.Children = oldChildren
					eventually.Assert(t, func(t require.TestingT) {
						dependencyGroup.verify(t, db)
					}, 20*time.Second, 200*time.Millisecond)
				})
			}
		})

		// TODO(jb): add tests for remaining config object types
	})
}

// waitForDumpDoneSignal reads from the icinga:dump Redis stream until there is a signal for key="*" state="done",
// that is icinga2 signals that it has completed its initial config dump.
func waitForDumpDoneSignal(t *testing.T, r services.RedisServer, wait time.Duration, interval time.Duration) {
	rc := r.Open()
	defer func() { _ = rc.Close() }()

	require.Eventually(t, func() bool {
		stream, err := rc.XRead(context.Background(), &redis.XReadArgs{
			Streams: []string{"icinga:dump", "0-0"},
			Block:   -1,
		}).Result()
		if err == redis.Nil {
			// empty stream
			return false
		}
		require.NoError(t, err, "redis xread should succeed")

		for _, message := range stream[0].Messages {
			key, ok := message.Values["key"]
			require.Truef(t, ok, "icinga:dump message should contain 'key': %+v", message)

			state, ok := message.Values["state"]
			require.Truef(t, ok, "icinga:dump message should contain 'state': %+v", message)

			if key == "*" && state == "done" {
				return true
			}
		}

		return false
	}, wait, interval, "icinga2 should signal key='*' state='done'")
}

type Host struct {
	Name                  string             `                                  icingadb:"name"`
	DisplayName           string             `icinga2:"display_name"            icingadb:"display_name"`
	Address               string             `icinga2:"address"                 icingadb:"address"`
	Address6              string             `icinga2:"address6"                icingadb:"address6"`
	CheckCommandName      string             `icinga2:"check_command"           icingadb:"checkcommand_name"`
	MaxCheckAttempts      float64            `icinga2:"max_check_attempts"      icingadb:"max_check_attempts"`
	CheckPeriodName       string             `icinga2:"check_period"            icingadb:"check_timeperiod_name"`
	CheckTimeout          float64            `icinga2:"check_timeout"           icingadb:"check_timeout"`
	CheckInterval         float64            `icinga2:"check_interval"          icingadb:"check_interval"`
	RetryInterval         float64            `icinga2:"retry_interval"          icingadb:"check_retry_interval"`
	EnableNotifications   bool               `icinga2:"enable_notifications"    icingadb:"notifications_enabled"`
	EnableActiveChecks    bool               `icinga2:"enable_active_checks"    icingadb:"active_checks_enabled"`
	EnablePassiveChecks   bool               `icinga2:"enable_passive_checks"   icingadb:"passive_checks_enabled"`
	EnableEventHandler    bool               `icinga2:"enable_event_handler"    icingadb:"event_handler_enabled"`
	EnableFlapping        bool               `icinga2:"enable_flapping"         icingadb:"flapping_enabled"`
	FlappingThresholdHigh float64            `icinga2:"flapping_threshold_high" icingadb:"flapping_threshold_high"`
	FlappingThresholdLow  float64            `icinga2:"flapping_threshold_low"  icingadb:"flapping_threshold_low"`
	EnablePerfdata        bool               `icinga2:"enable_perfdata"         icingadb:"perfdata_enabled"`
	EventCommandName      string             `icinga2:"event_command"           icingadb:"eventcommand_name"`
	Volatile              bool               `icinga2:"volatile"                icingadb:"is_volatile"`
	ZoneName              string             `icinga2:"zone"                    icingadb:"zone_name"`
	CommandEndpointName   string             `icinga2:"command_endpoint"        icingadb:"command_endpoint_name"`
	Notes                 string             `icinga2:"notes"                   icingadb:"notes"`
	NotesUrl              *string            `icinga2:"notes_url"               icingadb:"notes_url.notes_url"`
	ActionUrl             *string            `icinga2:"action_url"              icingadb:"action_url.action_url"`
	IconImage             *string            `icinga2:"icon_image"              icingadb:"icon_image.icon_image"`
	IconImageAlt          string             `icinga2:"icon_image_alt"          icingadb:"icon_image_alt"`
	Vars                  *CustomVarTestData `icinga2:"vars"`
	// TODO(jb): groups

	utils.VariantInfo
}

func makeTestSyncHosts(t *testing.T) []Host {
	host := Host{
		Address:               "127.0.0.1",
		Address6:              "::1",
		CheckCommandName:      "hostalive",
		MaxCheckAttempts:      3,
		CheckTimeout:          60,
		CheckInterval:         10,
		RetryInterval:         5,
		FlappingThresholdHigh: 80,
		FlappingThresholdLow:  20,
	}

	hosts := utils.MakeVariants(host).
		Vary("DisplayName", "Some Display Name", "Other Display Name").
		Vary("Address", "192.0.2.23", "192.0.2.42").
		Vary("Address6", "2001:db8::23", "2001:db8::42").
		Vary("CheckCommandName", "some-checkcommand", "other-checkcommand").
		Vary("MaxCheckAttempts", 5.0, 7.0).
		Vary("CheckPeriodName", "some-timeperiod", "other-timeperiod").
		Vary("CheckTimeout", 30. /* TODO(jb): 5 */, 120.0).
		Vary("CheckInterval", 20. /* TODO(jb): 5 */, 30.0).
		Vary("RetryInterval", 1. /* TODO(jb): 5 */, 2.0).
		Vary("EnableNotifications", true, false).
		Vary("EnableActiveChecks", true, false).
		Vary("EnablePassiveChecks", true, false).
		Vary("EnableEventHandler", true, false).
		Vary("EnableFlapping", true, false).
		Vary("FlappingThresholdHigh", 90.0, 95.5).
		Vary("FlappingThresholdLow", 5.5, 10.0).
		Vary("EnablePerfdata", true, false).
		Vary("EventCommandName", "some-eventcommand", "other-eventcommand").
		Vary("Volatile", true, false).
		Vary("ZoneName", "some-zone", "other-zone").
		Vary("CommandEndpointName", "some-endpoint", "other-endpoint").
		Vary("Notes", "Some Notes", "Other Notes").
		Vary("NotesUrl", newString("https://some.notes.invalid/host"), newString("http://other.notes.invalid/host")).
		Vary("ActionUrl", newString("https://some.action.invalid/host"), newString("http://other.actions.invalid/host")).
		Vary("IconImage", newString("https://some.icon.invalid/host.png"), newString("http://other.icon.invalid/host.jpg")).
		Vary("IconImageAlt", "Some Icon Image Alt", "Other Icon Image Alt").
		Vary("Vars", localutils.AnySliceToInterfaceSlice(makeCustomVarTestData(t))...).
		ResultAsBaseTypeSlice().([]Host)

	for i := range hosts {
		hosts[i].Name = utils.UniqueName(t, "host")

		if hosts[i].DisplayName == "" {
			// if no display_name is set, it defaults to name
			hosts[i].DisplayName = hosts[i].Name
		}

		if hosts[i].ZoneName == "" {
			hosts[i].ZoneName = "master"
		}
	}

	return hosts
}

type Service struct {
	Name                  string             `                                  icingadb:"name"`
	DisplayName           string             `icinga2:"display_name"            icingadb:"display_name"`
	HostName              *string            `icinga2:"host_name,nomodify"      icingadb:"host.name"`
	CheckCommandName      string             `icinga2:"check_command"           icingadb:"checkcommand_name"`
	MaxCheckAttempts      float64            `icinga2:"max_check_attempts"      icingadb:"max_check_attempts"`
	CheckPeriodName       string             `icinga2:"check_period"            icingadb:"check_timeperiod_name"`
	CheckTimeout          float64            `icinga2:"check_timeout"           icingadb:"check_timeout"`
	CheckInterval         float64            `icinga2:"check_interval"          icingadb:"check_interval"`
	RetryInterval         float64            `icinga2:"retry_interval"          icingadb:"check_retry_interval"`
	EnableNotifications   bool               `icinga2:"enable_notifications"    icingadb:"notifications_enabled"`
	EnableActiveChecks    bool               `icinga2:"enable_active_checks"    icingadb:"active_checks_enabled"`
	EnablePassiveChecks   bool               `icinga2:"enable_passive_checks"   icingadb:"passive_checks_enabled"`
	EnableEventHandler    bool               `icinga2:"enable_event_handler"    icingadb:"event_handler_enabled"`
	EnableFlapping        bool               `icinga2:"enable_flapping"         icingadb:"flapping_enabled"`
	FlappingThresholdHigh float64            `icinga2:"flapping_threshold_high" icingadb:"flapping_threshold_high"`
	FlappingThresholdLow  float64            `icinga2:"flapping_threshold_low"  icingadb:"flapping_threshold_low"`
	EnablePerfdata        bool               `icinga2:"enable_perfdata"         icingadb:"perfdata_enabled"`
	EventCommandName      string             `icinga2:"event_command"           icingadb:"eventcommand_name"`
	Volatile              bool               `icinga2:"volatile"                icingadb:"is_volatile"`
	ZoneName              string             `icinga2:"zone"                    icingadb:"zone_name"`
	CommandEndpointName   string             `icinga2:"command_endpoint"        icingadb:"command_endpoint_name"`
	Notes                 string             `icinga2:"notes"                   icingadb:"notes"`
	NotesUrl              *string            `icinga2:"notes_url"               icingadb:"notes_url.notes_url"`
	ActionUrl             *string            `icinga2:"action_url"              icingadb:"action_url.action_url"`
	IconImage             *string            `icinga2:"icon_image"              icingadb:"icon_image.icon_image"`
	IconImageAlt          string             `icinga2:"icon_image_alt"          icingadb:"icon_image_alt"`
	Vars                  *CustomVarTestData `icinga2:"vars"`
	// TODO(jb): groups

	utils.VariantInfo
}

func makeTestSyncServices(t *testing.T) []Service {
	service := Service{
		HostName:              newString("default-host"),
		CheckCommandName:      "default-checkcommand",
		MaxCheckAttempts:      3,
		CheckTimeout:          60,
		CheckInterval:         10,
		RetryInterval:         5,
		EnableNotifications:   true,
		EnableActiveChecks:    true,
		EnablePassiveChecks:   true,
		EnableEventHandler:    true,
		EnableFlapping:        true,
		FlappingThresholdHigh: 80,
		FlappingThresholdLow:  20,
		EnablePerfdata:        true,
		Volatile:              false,
	}

	services := utils.MakeVariants(service).
		Vary("HostName", newString("some-host"), newString("other-host")).
		Vary("DisplayName", "Some Display Name", "Other Display Name").
		Vary("CheckCommandName", "some-checkcommand", "other-checkcommand").
		Vary("MaxCheckAttempts", 5.0, 7.0).
		Vary("CheckPeriodName", "some-timeperiod", "other-timeperiod").
		Vary("CheckTimeout", 23.0 /* TODO(jb): .42 */, 120.0).
		Vary("CheckInterval", 20.0, 42.0 /* TODO(jb): .23 */).
		Vary("RetryInterval", 1.0 /* TODO(jb): .5 */, 15.0).
		Vary("EnableNotifications", true, false).
		Vary("EnableActiveChecks", true, false).
		Vary("EnablePassiveChecks", true, false).
		Vary("EnableEventHandler", true, false).
		Vary("EnableFlapping", true, false).
		Vary("FlappingThresholdHigh", 95.0, 99.5).
		Vary("FlappingThresholdLow", 0.5, 10.0).
		Vary("EnablePerfdata", true, false).
		Vary("EventCommandName", "some-eventcommand", "other-eventcommand").
		Vary("Volatile", true, false).
		Vary("ZoneName", "some-zone", "other-zone").
		Vary("CommandEndpointName", "some-endpoint", "other-endpoint").
		Vary("Notes", "Some Notes", "Other Notes").
		Vary("NotesUrl", newString("https://some.notes.invalid/service"), newString("http://other.notes.invalid/service")).
		Vary("ActionUrl", newString("https://some.action.invalid/service"), newString("http://other.actions.invalid/service")).
		Vary("IconImage", newString("https://some.icon.invalid/service.png"), newString("http://other.icon.invalid/service.jpg")).
		Vary("IconImageAlt", "Some Icon Image Alt", "Other Icon Image Alt").
		Vary("Vars", localutils.AnySliceToInterfaceSlice(makeCustomVarTestData(t))...).
		ResultAsBaseTypeSlice().([]Service)

	for i := range services {
		services[i].Name = utils.UniqueName(t, "service")

		if services[i].DisplayName == "" {
			// if no display_name is set, it defaults to name
			services[i].DisplayName = services[i].Name
		}

		if services[i].ZoneName == "" {
			services[i].ZoneName = "master"
		}
	}

	return services
}

type User struct {
	// TODO(jb): vars, groups
	Name                string                   `                               icingadb:"name"`
	DisplayName         string                   `icinga2:"display_name"         icingadb:"display_name"`
	Email               string                   `icinga2:"email"                icingadb:"email"`
	Pager               string                   `icinga2:"pager"                icingadb:"pager"`
	EnableNotifications bool                     `icinga2:"enable_notifications" icingadb:"notifications_enabled"`
	Period              *string                  `icinga2:"period"               icingadb:"timeperiod.name"`
	Types               value.NotificationTypes  `icinga2:"types"                icingadb:"types"`
	States              value.NotificationStates `icinga2:"states"               icingadb:"states"`

	utils.VariantInfo
}

func makeTestUsers(t *testing.T) []User {
	users := utils.MakeVariants(User{EnableNotifications: true}).
		Vary("DisplayName", "Some Display Name", "Other Display Name").
		Vary("Email", "some@email.invalid", "other@email.invalid").
		Vary("Pager", "some pager", "other pager").
		Vary("EnableNotifications", true, false).
		Vary("Period", newString("some-timeperiod"), newString("other-timeperiod")).
		Vary("Types",
			value.NotificationTypes{"DowntimeStart", "DowntimeEnd", "DowntimeRemoved"},
			value.NotificationTypes{"Custom"},
			value.NotificationTypes{"Acknowledgement"},
			value.NotificationTypes{"Problem", "Recovery"},
			value.NotificationTypes{"FlappingStart", "FlappingEnd"},
			value.NotificationTypes{"DowntimeStart", "Problem", "FlappingStart"},
			value.NotificationTypes{"DowntimeEnd", "DowntimeRemoved", "Recovery", "FlappingEnd"},
			value.NotificationTypes{"DowntimeStart", "DowntimeEnd", "DowntimeRemoved", "Custom", "Acknowledgement", "Problem", "Recovery", "FlappingStart", "FlappingEnd"},
			value.NotificationTypes{"Custom", "Acknowledgement"},
		).
		Vary("States",
			value.NotificationStates{},
			value.NotificationStates{"Up", "Down"},
			value.NotificationStates{"OK", "Warning", "Critical", "Unknown"},
			value.NotificationStates{"Critical", "Down"},
			value.NotificationStates{"OK", "Warning", "Critical", "Unknown", "Up", "Down"}).
		ResultAsBaseTypeSlice().([]User)

	for i := range users {
		users[i].Name = utils.UniqueName(t, "user")
		if users[i].DisplayName == "" {
			users[i].DisplayName = users[i].Name
		}
	}

	return users
}

type Notification struct {
	Name        string            `                                  icingadb:"name"`
	HostName    *string           `icinga2:"host_name,nomodify"      icingadb:"host.name"`
	ServiceName *string           `icinga2:"service_name,nomodify"   icingadb:"service.name"`
	Command     string            `icinga2:"command"                 icingadb:"notificationcommand.name"`
	Times       map[string]string `icinga2:"times"`
	Interval    int               `icinga2:"interval"                icingadb:"notification_interval"`
	Period      *string           `icinga2:"period"                  icingadb:"timeperiod.name"`
	//Zone      string                   `icinga2:"zone"                    icingadb:"zone.name"`
	Types      value.NotificationTypes  `icinga2:"types"                   icingadb:"types"`
	States     value.NotificationStates `icinga2:"states"                  icingadb:"states"`
	Users      []string                 `icinga2:"users"`
	UserGroups []string                 `icinga2:"user_groups"`
	Vars       *CustomVarTestData       `icinga2:"vars"`

	utils.VariantInfo
}

func (n Notification) fullName() string {
	if n.ServiceName == nil {
		return *n.HostName + "!" + n.Name
	} else {
		return *n.HostName + "!" + *n.ServiceName + "!" + n.Name
	}
}

func (n Notification) verify(t require.TestingT, db *sqlx.DB) {
	verifyIcingaDbRow(t, db, n)

	// Check if the "notification_user" table has been populated correctly
	{
		query := `SELECT u.name FROM notification n JOIN notification_user nu ON n.id = nu.notification_id JOIN "user" u ON u.id = nu.user_id WHERE n.name = ? ORDER BY u.name`
		var rows []string
		err := db.Select(&rows, db.Rebind(query), n.fullName())
		require.NoError(t, err, "SQL query")

		expected := append([]string(nil), n.Users...)
		sort.Strings(expected)

		assert.Equal(t, expected, rows, "Users in database should be equal")
	}

	// Check if the "notification_groups" table has been populated correctly
	{
		query := "SELECT ug.name FROM notification n JOIN notification_usergroup ng ON n.id = ng.notification_id JOIN usergroup ug ON ug.id = ng.usergroup_id WHERE n.name = ? ORDER BY ug.name"
		var rows []string
		err := db.Select(&rows, db.Rebind(query), n.fullName())
		require.NoError(t, err, "SQL query")

		expected := append([]string(nil), n.UserGroups...)
		sort.Strings(expected)
		require.Equal(t, expected, rows, "Usergroups in database should be equal")
	}

	// Check if the "notification_recipients" table has been populated correctly
	{
		type Row struct {
			User  *string `db:"username"`
			Group *string `db:"groupname"`
		}

		var expected []Row

		for _, user := range n.Users {
			expected = append(expected, Row{User: newString(user)})
		}

		for _, userGroup := range n.UserGroups {
			expected = append(expected, Row{Group: newString(userGroup)})
			for user, groups := range users {
				if _, ok := groups[userGroup]; ok {
					expected = append(expected, Row{User: newString(user), Group: newString(userGroup)})
				}
			}
		}

		sort.Slice(expected, func(i, j int) bool {
			r1 := expected[i]
			r2 := expected[j]

			stringComparePtr := func(a, b *string) int {
				if a == nil && b == nil {
					return 0
				} else if a == nil {
					return -1
				} else if b == nil {
					return 1
				}

				return strings.Compare(*a, *b)
			}

			switch stringComparePtr(r1.User, r2.User) {
			case -1:
				return true
			case 1:
				return false
			default:
				return stringComparePtr(r1.Group, r2.Group) == -1
			}
		})

		query := "SELECT u.name AS username, ug.name AS groupname FROM notification n " +
			"JOIN notification_recipient nr ON n.id = nr.notification_id " +
			`LEFT JOIN "user" u ON u.id = nr.user_id ` +
			"LEFT JOIN usergroup ug ON ug.id = nr.usergroup_id " +
			"WHERE n.name = ? " +
			"ORDER BY u.name IS NOT NULL, u.name, ug.name IS NOT NULL, ug.name"

		var rows []Row
		err := db.Select(&rows, db.Rebind(query), n.fullName())

		require.NoError(t, err, "SQL query")
		require.Equal(t, expected, rows, "Recipients in database should be equal")
	}
}

func makeTestNotifications(t *testing.T) []Notification {
	notification := Notification{
		HostName:    newString("default-host"),
		ServiceName: newString("default-service"),
		Command:     "default-notificationcommand",
		Users:       []string{"default-user", "testuser1", "testuser2", "testuser3"},
		UserGroups:  []string{"default-usergroup", "testusergroup1", "testusergroup2", "testusergroup3"},
		Interval:    1800,
	}

	notifications := utils.MakeVariants(notification).
		//Vary("TimesBegin", 5, 999, 23980, 525, 666, 0).
		//Vary("TimesEnd", 0, 453, 74350, 423, 235, 63477).
		Vary("Interval", 5, 453, 74350, 423, 235, 63477).
		Vary("Period", newString("some-timeperiod"), newString("other-timeperiod")).
		Vary("Types",
			value.NotificationTypes{"DowntimeStart", "DowntimeEnd", "DowntimeRemoved"},
			value.NotificationTypes{"Custom"},
			value.NotificationTypes{"Acknowledgement"},
			value.NotificationTypes{"Problem", "Recovery"},
			value.NotificationTypes{"FlappingStart", "FlappingEnd"},
			value.NotificationTypes{"DowntimeStart", "Problem", "FlappingStart"},
			value.NotificationTypes{"DowntimeEnd", "DowntimeRemoved", "Recovery", "FlappingEnd"},
			value.NotificationTypes{"DowntimeStart", "DowntimeEnd", "DowntimeRemoved", "Custom", "Acknowledgement", "Problem", "Recovery", "FlappingStart", "FlappingEnd"},
			value.NotificationTypes{"Custom", "Acknowledgement"},
		).
		Vary("States",
			value.NotificationStates{},
			value.NotificationStates{"OK", "Warning", "Critical", "Unknown"},
			value.NotificationStates{"OK", "Unknown"}).
		Vary("Users", localutils.AnySliceToInterfaceSlice(localutils.SliceSubsets(
			"default-user", "some-user", "other-user", "testuser1", "testuser2", "testuser3"))...).
		Vary("UserGroups", localutils.AnySliceToInterfaceSlice(localutils.SliceSubsets(
			"default-usergroup", "some-usergroup", "other-usergroup", "testusergroup1", "testusergroup2", "testusergroup3"))...).
		ResultAsBaseTypeSlice().([]Notification)

	for i := range notifications {
		notifications[i].Name = utils.UniqueName(t, "notification")
	}

	return notifications
}

type DependencyGroup struct {
	Id               types.Binary `db:"id"`
	Name             string       `db:"display_name"`
	Parents          []string
	Children         []string
	StatesFilter     []string
	TimePeriod       string
	IgnoreSoftStates bool

	SkipVerification   bool
	SendRuntimeUpdates bool
}

func (g *DependencyGroup) IsRedundancyGroup() bool {
	return g.Name != ""
}

func (g *DependencyGroup) InfoString() string {
	prefix := "Non-Redundant"
	if g.IsRedundancyGroup() {
		prefix = g.Name
	}

	if len(g.Children) == 0 {
		return prefix + "-" + strings.Join(g.Parents, "-") + "--" + "NoChildren"
	}

	return prefix + "-" + strings.Join(g.Parents, "-") + "--" + strings.Join(g.Children, "-")
}

// assertNoDependencyDanglingReferences verifies that there are no dangling references in the dependency_node,
// dependency_edge and redundancy_group_state tables.
//
// Since the dependency group tests are executed in parallel, there is a chance that the database becomes inconsistent
// for a split second, which can be detected by this function and will cause the test to fail. So, calling this function
// only once after all tests have finished is sufficient.
func assertNoDependencyDanglingReferences(t require.TestingT, r services.RedisServer, db *sqlx.DB) {
	rc := r.Open()
	defer func() { _ = rc.Close() }()

	redisHGetCheck := func(key, field string) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		result, err := rc.HGet(ctx, key, field).Result()
		if err != nil {
			assert.Equal(t, redis.Nil, err)
		}
		assert.Emptyf(t, result, "%s %q exists in Redis but not in the database", strings.Split(key, ":")[1], field)
	}

	var nodes []struct {
		HostID            types.Binary `db:"host_id"`
		ServiceID         types.Binary `db:"service_id"`
		RedundancyGroupID types.Binary `db:"redundancy_group_id"`
	}
	err := db.Select(&nodes, `SELECT host_id, service_id, redundancy_group_id FROM dependency_node`)
	require.NoError(t, err, "querying dependency nodes")

	// Check if there are any dangling references in the dependency_node table, i.e. nodes that reference
	// unknown hosts, services or redundancy groups.
	for _, node := range nodes {
		var exists bool
		if node.HostID != nil {
			assert.Nilf(t, node.RedundancyGroupID, "node redudancy group ID should be nil if host ID is set")

			err := db.Get(&exists, db.Rebind(`SELECT EXISTS (SELECT 1 FROM host WHERE id = ?)`), node.HostID)
			assert.NoError(t, err, "querying host existence")
			assert.Truef(t, exists, "host %q should exist", node.HostID)

			if !exists {
				redisHGetCheck("icinga:host", node.HostID.String())
			}
		}

		if node.ServiceID != nil {
			assert.NotNil(t, node.HostID, "node host ID should be set if service ID is set")
			assert.Nilf(t, node.RedundancyGroupID, "node redudancy group ID should be nil if service ID is set")

			err := db.Get(&exists, db.Rebind(`SELECT EXISTS (SELECT 1 FROM service WHERE id = ?)`), node.ServiceID)
			assert.NoError(t, err, "querying service existence")
			assert.Truef(t, exists, "service %q should exist", node.ServiceID)

			if !exists {
				redisHGetCheck("icinga:service", node.ServiceID.String())
			}
		}

		if node.RedundancyGroupID != nil {
			assert.Nilf(t, node.HostID, "node host ID should be nil if redundancy group ID is set")
			assert.Nilf(t, node.ServiceID, "node service ID should be nil if redundancy group ID is set")

			err := db.Get(&exists, db.Rebind(`SELECT EXISTS (SELECT 1 FROM redundancy_group WHERE id = ?)`), node.RedundancyGroupID)
			assert.NoError(t, err, "querying redundancy group existence")
			assert.Truef(t, exists, "redundancy group %q should exist", node.RedundancyGroupID)

			if !exists {
				redisHGetCheck("icinga:redundancygroup", node.RedundancyGroupID.String())
			}
		}
	}

	var edges []struct {
		FromNodeID types.Binary `db:"from_node_id"`
		ToNodeID   types.Binary `db:"to_node_id"`
		StateID    types.Binary `db:"dependency_edge_state_id"`
	}
	err = db.Select(&edges, `SELECT from_node_id, to_node_id, dependency_edge_state_id FROM dependency_edge`)
	require.NoError(t, err, "querying dependency edges")

	// Check if there are any dangling references in the dependency_edge table, i.e. edges that reference
	// unknown from/to nodes or dependency edge states.
	for _, edge := range edges {
		assert.NotNil(t, edge.FromNodeID, "from node ID should be set")
		assert.NotNil(t, edge.ToNodeID, "to node ID should be set")
		assert.NotNil(t, edge.StateID, "dependency edge state ID should be set")

		var exists bool
		err := db.Get(&exists, db.Rebind(`SELECT EXISTS (SELECT 1 FROM dependency_node WHERE id = ?)`), edge.FromNodeID)
		assert.NoError(t, err, "querying child/from node existence")
		assert.Truef(t, exists, "child/from node %q should exist", edge.FromNodeID)

		if !exists {
			redisHGetCheck("icinga:dependency:node", edge.FromNodeID.String())
		}

		err = db.Get(&exists, db.Rebind(`SELECT EXISTS (SELECT 1 FROM dependency_node WHERE id = ?)`), edge.ToNodeID)
		assert.NoError(t, err, "querying parent/to node existence")
		assert.Truef(t, exists, "parent/to node %q should exist", edge.ToNodeID)

		if !exists {
			redisHGetCheck("icinga:dependency:node", edge.ToNodeID.String())
		}

		err = db.Get(&exists, db.Rebind(`SELECT EXISTS (SELECT 1 FROM dependency_edge_state WHERE id = ?)`), edge.StateID)
		assert.NoError(t, err, "querying dependency edge state existence")
		assert.Truef(t, exists, "dependency edge state %q should exist", edge.StateID)

		if !exists {
			redisHGetCheck("icinga:dependency:edge:state", edge.StateID.String())
		}
	}

	// TODO: Icinga 2 does send runtime delete event for those two tables, but Icinga DB does not handle them well yet.
	//  This is because the runtime sync pipeline is set to process upsert/delete events concurrently, which can sometimes
	//  lead to race conditions where the two events are processed in the wrong order.
	/*var stateIDs []types.Binary
	err = db.Select(&stateIDs, `SELECT id FROM dependency_edge_state WHERE id NOT IN (SELECT dependency_edge_state_id FROM dependency_edge)`)
	assert.NoError(t, err, "querying dangling dependency edge states")
	assert.Len(t, stateIDs, 0, "all dependency_edge_state IDs should be referenced by a dependency_edge")

	for _, stateID := range stateIDs {
		// Check if these dangling state IDs are still present in Redis.
		redisHGetCheck("icinga:dependency:edge:state", stateID.String())
	}

	stateIDs = nil
	// Verify that all redundancy group states do reference an existing redundancy group.
	err = db.Select(&stateIDs, `SELECT id FROM redundancy_group_state WHERE id NOT IN (SELECT id FROM redundancy_group)`)
	assert.NoError(t, err, "querying dangling redundancy group states")
	assert.Len(t, stateIDs, 0, "redundancy_group_state referencing unknown redundancy groups")

	for _, stateID := range stateIDs {
		// Check if these dangling state IDs are still present in Redis.
		redisHGetCheck("icinga:redundancygroup:state", stateID.String())
	}*/
}

// verify performs a series of checks to ensure that the dependency group is correctly represented in the database.
//
// The following checks are performed:
//   - Verify that the redundancy group (if any) is referenced by the children hosts as their parent node.
//   - Verify that all child and parent Checkables of this group are in the regular Icinga DB tables and the redundancy
//     group (if any) is in the redundancy_group table as well as having a state in the redundancy_group_state table.
//   - Verify that the dependency_node and dependency_edge tables are correctly populated with the expected from/to
//     nodes according to the group's configuration. This includes verifying the connection between the child Checkables
//     and the redundancy group (if any) and from redundancy group to parent Checkables.
//
// To simplify the verification process, dependency groups that have the SkipVerification flag set are not
// verified, as they are merged into a single edge by Icinga 2, instead we're going to verify their original
// counterpart (the group they were merged into).
func (g *DependencyGroup) verify(t require.TestingT, db *sqlx.DB) {
	if g.SkipVerification {
		// All duplicated dependency objects get merged into a single edge by Icinga 2, so we don't need to verify
		// such groups here, as we're going to verify the original group (the one this group was merged into) instead.
		return
	}

	if len(g.Children) == 0 {
		if g.IsRedundancyGroup() {
			require.NotNilf(t, g.Id, "ID should be set for redundancy group %q", g.InfoString())

			var exists bool
			err := db.Get(&exists, db.Rebind(`SELECT EXISTS(SELECT 1 FROM redundancy_group WHERE id = ?)`), g.Id)
			assert.NoErrorf(t, err, "fetching redundancy group %q with ID %q", g.InfoString(), g.Id)
			assert.Falsef(t, exists, "redundancy group %q with ID %q should not exist", g.InfoString(), g.Id)

			err = db.Get(&exists, db.Rebind(`SELECT EXISTS(SELECT 1 FROM redundancy_group_state WHERE id = ?)`), g.Id)
			assert.NoErrorf(t, err, "fetching redundancy group state by ID %q", g.Id)
			assert.Falsef(t, exists, "redundancy group state with ID %q should not exist", g.Id)
		}

		return
	}

	// Fetch the redundancy group referenced by the children hosts as their parent node (if any).
	// If the dependency group is a redundancy group, the redundancy group itself should be the parent node
	// and if the dependency serialization is correct, we should find only a single redundancy group.
	query, args, err := sqlx.In(`SELECT rg.id, rg.display_name FROM redundancy_group rg
		INNER JOIN dependency_node parent ON parent.redundancy_group_id = rg.id
		INNER JOIN dependency_edge tn ON tn.to_node_id = parent.id
		INNER JOIN dependency_node child ON child.id = tn.from_node_id
		INNER JOIN host child_host ON child_host.id = child.host_id
		INNER JOIN redundancy_group_state rgs ON rgs.redundancy_group_id = rg.id
		WHERE child_host.name IN (?)`,
		g.Children,
	)
	require.NoError(t, err, "expanding SQL IN clause for redundancy groups query")

	var redundancyGroups []DependencyGroup
	err = db.Select(&redundancyGroups, db.Rebind(query+` AND rg.display_name = ? GROUP BY rg.id, rg.display_name`), append(args, g.Name)...)
	require.NoError(t, err, "fetching redundancy groups")

	var redundancyGroup DependencyGroup
	if g.IsRedundancyGroup() {
		require.Lenf(t, redundancyGroups, 1, "there should be exactly one redundancy group %q", g.InfoString())
		redundancyGroup = redundancyGroups[0]
		g.Id = redundancyGroup.Id
		assert.Equal(t, g.Name, redundancyGroup.Name, "redundancy group name should match")
	} else {
		assert.Lenf(t, redundancyGroups, 0, "there should be no redundancy group %q", g.InfoString())
	}

	type Checkable struct {
		NodeId      types.Binary `db:""`
		EdgeStateId types.Binary `db:"-"`
		Id          types.Binary `db:"id"`
		Name        string       `db:"name"`
	}

	// Perform some basic sanity checks on the hosts and redundancy groups (if any).
	query, args, err = sqlx.In("SELECT id, name FROM host WHERE name IN (?)", append(append([]string(nil), g.Parents...), g.Children...))
	require.NoError(t, err, "expanding SQL IN clause for hosts query")

	hostRows, err := db.Queryx(db.Rebind(query), args...)
	require.NoError(t, err, "querying parent and child hosts")
	defer hostRows.Close()

	checkables := make(map[string]*Checkable)
	for hostRows.Next() {
		var c Checkable
		require.NoError(t, hostRows.StructScan(&c), "scanning host row")
		checkables[c.Name] = &c

		// Retrieve the dependency node and dependency edge state ID of the current host.
		assert.NoError(t, db.Get(&c.NodeId, db.Rebind(`SELECT id FROM dependency_node WHERE host_id = ?`), c.Id))
		assert.NotNilf(t, c.NodeId, "host %q should have a dependency node", c.Name)

		if slices.Contains(g.Children, c.Name) && g.IsRedundancyGroup() {
			assert.NoError(t, db.Get(&c.EdgeStateId, db.Rebind(`SELECT dependency_edge_state_id FROM dependency_edge WHERE to_node_id = ?`), redundancyGroup.Id))
			assert.NotNilf(t, c.EdgeStateId, "host %q should have a dependency edge state", c.Name)
		}
	}
	assert.NoError(t, hostRows.Err(), "scanned host rows should not have errors")
	assert.Len(t, checkables, len(g.Parents)+len(g.Children), "all hosts should be in the database")

	type Node struct {
		RedundancyGroupId types.Binary `db:"node_id"`
		Name              string       `db:"name"`
		FromNodeId        types.Binary `db:"from_node_id"`
		ToNodeId          types.Binary `db:"to_node_id"`
	}

	// Retrieve all parent nodes (including this redundancy group) referenced by the children hosts of this dependency group.
	query, args, err = sqlx.In(`SELECT rg.id AS node_id, COALESCE(h.name, rg.display_name) AS name, from_node_id, to_node_id
		FROM dependency_node dn
			LEFT JOIN host h ON h.id = dn.host_id
			LEFT JOIN dependency_edge tn ON tn.to_node_id = dn.id
			LEFT JOIN redundancy_group rg ON rg.id = dn.redundancy_group_id
		WHERE EXISTS(SELECT 1
			FROM dependency_edge de
				INNER JOIN dependency_node parent ON parent.id = de.to_node_id
				INNER JOIN dependency_node child ON child.id = de.from_node_id
				LEFT JOIN host ch ON ch.id = child.host_id
			WHERE ch.name IN (?) AND parent.id = dn.id)`,
		g.Children,
	)
	require.NoError(t, err, "expanding SQL IN clause for parent nodes query")

	parentNodes := make(map[string]Node)
	dbParentNodes, err := db.Queryx(db.Rebind(query), args...)
	require.NoError(t, err, "querying parent nodes")
	defer dbParentNodes.Close()

	childToRedundancyGroupIds := make(map[string]bool)
	childToParentCheckableIds := make(map[string]map[string]bool)
	for dbParentNodes.Next() {
		var node Node
		require.NoError(t, dbParentNodes.StructScan(&node), "scanning parent node row")

		// The SQL query yields all parent nodes of the children hosts of this dependency group but also those from
		// any other redundancy group. Thus, we need to filter out all parent nodes that are not part of this group.
		if g.IsRedundancyGroup() && node.Name == g.Name {
			parentNodes[node.Name] = node
			// Cache the from_node_id of these retrieved parent nodes (this redundancy group), as we need to verify
			// that these IDs represent those of the children hosts of this group.
			childToRedundancyGroupIds[node.FromNodeId.String()] = true
		} else if !g.IsRedundancyGroup() && slices.Contains(g.Parents, node.Name) {
			parentNodes[node.Name] = node
			// Cache the from_node_id of these retrieved parent nodes (the parent hosts), as we need to
			// verify that these IDs represent those of the children hosts of this group.
			if _, ok := childToParentCheckableIds[node.FromNodeId.String()]; !ok {
				childToParentCheckableIds[node.FromNodeId.String()] = make(map[string]bool)
			}
			childToParentCheckableIds[node.FromNodeId.String()][node.ToNodeId.String()] = true
		}
	}
	assert.NoError(t, dbParentNodes.Err(), "scanned parent node rows should not have errors")

	expectedParentCount := len(g.Parents)
	if g.IsRedundancyGroup() {
		expectedParentCount = 1 // All the children should have the redundancy group as parent node!
		assert.Lenf(t, childToRedundancyGroupIds, len(g.Children), "all children %v should have the redundancy group as parent node", g.Children)
		for _, child := range g.Children {
			h := checkables[child]
			require.NotNil(t, h, "child node should be a Checkable")
			assert.Truef(t, childToRedundancyGroupIds[h.NodeId.String()], "child node %q should have the redundancy group as parent node", child)
			// The edge state ID of all the children of this redundancy group should be the same as the ID
			// of the redundancy group ID, i.e. they all share the same edge state. This is just a duplicate check!
			assert.Equalf(t, redundancyGroup.Id, h.EdgeStateId, "child node %q should have the correct edge state", child)
		}
	} else {
		for _, child := range g.Children {
			h := checkables[child]
			require.NotNilf(t, h, "child node %q should be a Checkable", child)

			parents := childToParentCheckableIds[h.NodeId.String()]
			require.NotNilf(t, parents, "child node %q should have parent nodes", child)
			assert.Lenf(t, parents, len(g.Parents), "child node %q should reference %d parent nodes", child, len(g.Parents))

			// Verify that the parent nodes of the children hosts are the correct ones.
			for _, p := range g.Parents {
				parent := checkables[p]
				require.NotNilf(t, parent, "parent node %q should be an existing Checkable", p)
				assert.Truef(t, parents[parent.NodeId.String()], "child node %q should reference parent node %q", child, parent)
			}
		}
	}
	assert.Lenf(t, parentNodes, expectedParentCount, "all parent nodes '%v' should be in the database", g.Parents)

	for _, node := range parentNodes {
		assert.NotNilf(t, node.FromNodeId, "parent node %q should have a from_node_id set", node.Name)
		assert.NotNilf(t, node.ToNodeId, "parent node %q should have a to_node_id set", node.Name)

		if g.IsRedundancyGroup() {
			assert.Equal(t, node.Name, g.Name, "parent node should be the redundancy group itself")
			assert.Equal(t, redundancyGroup.Id, node.RedundancyGroupId, "parent node should reference the redundancy group")

			// Verify whether the connection between the current redundancy group and the parent Checkable is correct.
			query := `SELECT NULL AS node_id, '' AS name, from_node_id, to_node_id FROM dependency_edge WHERE from_node_id = ?`
			var edges []Node
			assert.NoError(t, db.Select(&edges, db.Rebind(query), redundancyGroup.Id))
			assert.GreaterOrEqualf(t, len(edges), len(g.Parents), "redundancy group %q should have at least %d parent nodes", g.Name, len(g.Parents))

			// Due to different group registrations in makeTestDependencyGroups(), we might have more parent nodes
			// than those within this specific group. Thus, we just need to make sure that all our parent nodes are
			// referenced by this redundancy group.
			for _, parent := range g.Parents {
				h := checkables[parent]
				require.NotNil(t, h, "parent node should be an existing Checkable")
				assert.Truef(t, slices.ContainsFunc(edges, func(edge Node) bool {
					return bytes.Equal(h.NodeId, edge.ToNodeId)
				}), "redundancy group %q should reference parent node %q", g.Name, parent)
			}
		} else {
			assert.Falsef(t, node.RedundancyGroupId.Valid(), "non-redundant parent node %q should not reference a redundancy group", node.Name)
			assert.Contains(t, g.Parents, node.Name, "parent node should be in the parents list")
			assert.NotContains(t, g.Children, node.Name, "parent node should not be in the children list")

			parent := checkables[node.Name]
			require.NotNil(t, parent, "parent node should be an existing Checkable")
			assert.Equal(t, parent.Id, node.ToNodeId, "parent node should reference the correct Checkable")
		}
	}
}

// makeTestDependencyGroups generates a set of dependency groups that can be used for testing the dependency sync.
//
// All the parent and child Checkables used within this function are defined in the object_sync_test.conf file.
// Therefore, if you want to add some more dependency groups with new Checkables, you need to add them to the
// object_sync_test.conf file as well.
func makeTestDependencyGroups() []DependencyGroup {
	return []DependencyGroup{
		{
			Name:               "DNS",
			Parents:            []string{"HostA", "HostB"},
			Children:           []string{"HostC", "HostD", "HostE"},
			StatesFilter:       []string{"Up", "Down"},
			IgnoreSoftStates:   true,
			SendRuntimeUpdates: true,
		},
		// This is an exact duplicate of the first dependency group but only for child HostE and should
		// be merged into the above from HostE -> DNS dependency edge, i.e. no duplicates in the database.
		{
			Name:               "DNS",
			Parents:            []string{"HostA", "HostB"},
			Children:           []string{"HostE"},
			StatesFilter:       []string{"Up", "Down"},
			IgnoreSoftStates:   true,
			SendRuntimeUpdates: true,
			// Skip the verification of this group, as it's going to have more children and parents in it after merging.
			SkipVerification: true,
		},
		{
			Name:               "",
			Parents:            []string{"HostA"},
			Children:           []string{"HostF", "HostG", "HostH"},
			StatesFilter:       []string{"Up"},
			IgnoreSoftStates:   true,
			SendRuntimeUpdates: true,
		},
		// This is an exact duplicate of the above dependency group but only for child HostF and should
		// be merged into the above from HostF -> HostA dependency edge, i.e. no duplicates in the database.
		{
			Name:               "",
			Parents:            []string{"HostA"},
			Children:           []string{"HostF"},
			StatesFilter:       []string{"Up"},
			IgnoreSoftStates:   true,
			SendRuntimeUpdates: true,
			// Not easy to verify this one, as it gets merged into the above one with more children and parents in it.
			SkipVerification: true,
		},
		{
			Name:               "",
			Parents:            []string{"HostB"},
			Children:           []string{"HostF"},
			StatesFilter:       []string{"Down"},
			IgnoreSoftStates:   true,
			SendRuntimeUpdates: true,
		},
		// This is also an exact duplicate of the above one, but we want to register them separately to ease the
		// verification, i.e., we will create another HostG -> HostB dependency edge but with different config, so
		// that HostG gets an exclusive dependency edge to HostB.
		{ //... Start of the HostG -> HostB duplicates...
			Name:               "",
			Parents:            []string{"HostB"},
			Children:           []string{"HostG"},
			StatesFilter:       []string{"Down"},
			IgnoreSoftStates:   true,
			SendRuntimeUpdates: true,
		},
		//... and that's the aforementioned duplicated dependency edge for HostG -> HostB with ignore soft states set to false.
		{
			Name:               "",
			Parents:            []string{"HostB"},
			Children:           []string{"HostG"},
			StatesFilter:       []string{"Down"},
			IgnoreSoftStates:   false,
			SendRuntimeUpdates: true,
		},
		//... and another one with different states filter as the two groups from above.
		{
			Name:               "",
			Parents:            []string{"HostB"},
			Children:           []string{"HostG"},
			StatesFilter:       []string{"Up"},
			IgnoreSoftStates:   false,
			SendRuntimeUpdates: true,
		},
		//... and another one with different states filter as the two groups from above.
		{
			Name:               "",
			Parents:            []string{"HostB"},
			Children:           []string{"HostG"},
			StatesFilter:       []string{"Up", "Down"},
			IgnoreSoftStates:   true,
			SendRuntimeUpdates: true,
		},
		//... and another one with completely different states filter and configured time period.
		{
			Name:               "",
			Parents:            []string{"HostB"},
			Children:           []string{"HostG"},
			StatesFilter:       []string{"Up", "Down"},
			IgnoreSoftStates:   true,
			TimePeriod:         "never-ever",
			SendRuntimeUpdates: true,
		}, //... End of the HostG -> HostB duplicates.
		{
			Name:               "",
			Parents:            []string{"HostB"},
			Children:           []string{"HostH"},
			StatesFilter:       []string{"Up", "Down"},
			IgnoreSoftStates:   true,
			SendRuntimeUpdates: true,
		},
		{
			Name:               "LDAP",
			Parents:            []string{"HostC", "HostD", "HostE"},
			Children:           []string{"HostF", "HostG", "HostH"},
			StatesFilter:       []string{"Up", "Down"},
			IgnoreSoftStates:   true,
			SendRuntimeUpdates: true,
		},
		{
			Name:             "LDAP",
			Parents:          []string{"HostC"},
			Children:         []string{"HostI", "HostJ"},
			StatesFilter:     []string{"Up", "Down"},
			IgnoreSoftStates: true,
		},
		{
			Name:             "LDAP",
			Parents:          []string{"HostD"},
			Children:         []string{"HostI", "HostJ"},
			StatesFilter:     []string{"Up", "Down"},
			IgnoreSoftStates: false,
		},
		{
			Name:             "LDAP",
			Parents:          []string{"HostE"},
			Children:         []string{"HostI", "HostJ"},
			StatesFilter:     []string{"Down"},
			IgnoreSoftStates: true,
			TimePeriod:       "never-ever",
		},
		{
			Name:             "SQL Servers",
			Parents:          []string{"HostF"},
			Children:         []string{"HostI"},
			StatesFilter:     []string{"Up"},
			IgnoreSoftStates: false,
			TimePeriod:       "never-ever",
		},
		{
			Name:             "SQL Servers",
			Parents:          []string{"HostF"},
			Children:         []string{"HostJ"},
			StatesFilter:     []string{"Up"},
			IgnoreSoftStates: true,
			TimePeriod:       "never-ever",
		},
		{
			Name:         "Web Servers",
			Parents:      []string{"HostC", "HostD"},
			Children:     []string{"HostK", "HostL", "HostM", "HostN"},
			StatesFilter: []string{"Up", "Down"},
			TimePeriod:   "workhours",
		},
		// This will be merged into the above group but since both have same children, the verification
		// result will be the same as the above group.
		{
			Name:             "Web Servers",
			Parents:          []string{"HostC", "HostD"},
			Children:         []string{"HostK", "HostL", "HostM", "HostN"},
			StatesFilter:     []string{"Up", "Down"},
			IgnoreSoftStates: true,
			TimePeriod:       "never-ever",
		},
		{
			Name:               "Mail Servers",
			Parents:            []string{"HostE", "HostF", "HostG"},
			Children:           []string{"HostK", "HostL", "HostM", "HostN"},
			StatesFilter:       []string{"Down"},
			IgnoreSoftStates:   true,
			SendRuntimeUpdates: true,
		},
	}
}

// writeIcinga2ConfigObjects emits config objects as icinga2 DSL to a writer
// based on the type of obj and its field having icinga2 struct tags.
func writeIcinga2ConfigObject(w io.Writer, obj interface{}) error {
	o := reflect.ValueOf(obj)
	name := o.FieldByName("Name").Interface()
	typ := o.Type()
	typeName := typ.Name()

	_, err := fmt.Fprintf(w, "object %s %s {\n", typeName, value.ToIcinga2Config(name))
	if err != nil {
		return err
	}

	for fieldIndex := 0; fieldIndex < typ.NumField(); fieldIndex++ {
		tag := typ.Field(fieldIndex).Tag.Get("icinga2")
		attr := strings.Split(tag, ",")[0]
		if attr != "" {
			if v := o.Field(fieldIndex).Interface(); v != nil {
				_, err := fmt.Fprintf(w, "\t%s = %s\n", attr, value.ToIcinga2Config(v))
				if err != nil {
					return err
				}
			}
		}
	}

	_, err = fmt.Fprintf(w, "}\n")
	return err
}

// makeIcinga2ApiAttributes generates a map that can be JSON marshaled and passed to the icinga2 API
// based on the type of obj and its field having icinga2 struct tags. Fields that are marked as "nomodify"
// (for example `icinga2:"host_name,nomodify"`) are omitted if the modify parameter is set to true.
func makeIcinga2ApiAttributes(obj interface{}, modify bool) map[string]interface{} {
	attrs := make(map[string]interface{})

	o := reflect.ValueOf(obj)
	typ := o.Type()
	for fieldIndex := 0; fieldIndex < typ.NumField(); fieldIndex++ {
		tag := typ.Field(fieldIndex).Tag.Get("icinga2")
		parts := strings.Split(tag, ",")
		attr := parts[0]
		flags := parts[1:]
		if attr == "" || (modify && slices.Contains(flags, "nomodify")) {
			continue
		}
		if val := o.Field(fieldIndex).Interface(); val != nil {
			attrs[attr] = value.ToIcinga2Api(val)
		}
	}

	return attrs
}

// verifyIcingaDbRow checks that the object given by obj is properly present in the SQL database. It checks compares all
// struct fields that have an icingadb tag set to the column name. It automatically joins tables if required.
func verifyIcingaDbRow(t require.TestingT, db *sqlx.DB, obj interface{}) {
	o := reflect.ValueOf(obj)
	name := o.FieldByName("Name").Interface()
	typ := o.Type()
	typeName := typ.Name()

	if notification, ok := obj.(Notification); ok {
		name = notification.fullName()
	}

	type ColumnValueExpected struct {
		Column   string
		Value    interface{}
		Expected interface{}
	}

	joinColumns := func(cs []ColumnValueExpected) string {
		var c []string
		for i := range cs {
			var quotedParts []string
			for _, part := range strings.Split(cs[i].Column, ".") {
				quotedParts = append(quotedParts, `"`+part+`"`)
			}
			c = append(c, strings.Join(quotedParts, "."))
		}
		return strings.Join(c, ", ")
	}

	scanSlice := func(cs []ColumnValueExpected) []interface{} {
		var vs []interface{}
		for i := range cs {
			vs = append(vs, cs[i].Value)
		}
		return vs
	}

	table := strings.ToLower(typeName)
	var columns []ColumnValueExpected
	joins := make(map[string]struct{})

	for fieldIndex := 0; fieldIndex < typ.NumField(); fieldIndex++ {
		if col := typ.Field(fieldIndex).Tag.Get("icingadb"); col != "" && col != "name" {
			if val := o.Field(fieldIndex).Interface(); val != nil {
				dbVal := value.ToIcingaDb(val)
				scanVal := reflect.New(reflect.TypeOf(dbVal)).Interface()
				if strings.Contains(col, ".") {
					parts := strings.SplitN(col, ".", 2)
					joins[parts[0]] = struct{}{}
				} else {
					col = table + "." + col
				}
				columns = append(columns, ColumnValueExpected{
					Column:   col,
					Value:    scanVal,
					Expected: dbVal,
				})
			}
		}
	}

	joinsQuery := ""
	for join := range joins {
		joinsQuery += fmt.Sprintf(` LEFT JOIN "%s" ON "%s"."id" = "%s"."%s_id"`, join, join, table, join)
	}

	query := fmt.Sprintf(`SELECT %s FROM "%s" %s WHERE "%s"."name" = ?`,
		joinColumns(columns), table, joinsQuery, table)
	rows, err := db.Query(db.Rebind(query), name)
	require.NoError(t, err, "SQL query: %s", query)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next(), "SQL query should return a row: %s", query)

	err = rows.Scan(scanSlice(columns)...)
	require.NoError(t, err, "SQL scan: %s", query)

	for _, col := range columns {
		got := reflect.ValueOf(col.Value).Elem().Interface()
		assert.Equalf(t, col.Expected, got, "%s should match", col.Column)
	}

	require.False(t, rows.Next(), "SQL query should return only one row: %s", query)
}

// newString allocates a new *string and initializes it. This helper function exists as
// there seems to be no way to achieve this within a single statement.
func newString(s string) *string {
	return &s
}

type CustomVarTestData struct {
	Value    interface{}               // Value to put into config or API
	Vars     map[string]string         // Expected values in customvar table
	VarsFlat map[string]sql.NullString // Expected values in customvar_flat table
}

func (c *CustomVarTestData) Icinga2ConfigValue() string {
	if c == nil {
		return value.ToIcinga2Config(nil)
	}
	return value.ToIcinga2Config(c.Value)
}

func (c *CustomVarTestData) Icinga2ApiValue() interface{} {
	if c == nil {
		return value.ToIcinga2Api(nil)
	}
	return value.ToIcinga2Api(c.Value)
}

func (c *CustomVarTestData) VerifyCustomVar(t require.TestingT, logger *zap.Logger, db *sqlx.DB, obj interface{}) {
	c.verify(t, logger, db, obj, false)
}

func (c *CustomVarTestData) VerifyCustomVarFlat(t require.TestingT, logger *zap.Logger, db *sqlx.DB, obj interface{}) {
	c.verify(t, logger, db, obj, true)
}

func (c *CustomVarTestData) verify(t require.TestingT, logger *zap.Logger, db *sqlx.DB, obj interface{}, flat bool) {
	o := reflect.ValueOf(obj)
	name := o.FieldByName("Name").Interface()
	typ := o.Type()
	typeName := typ.Name()
	table := strings.ToLower(typeName)

	query := ""
	if flat {
		query += "SELECT customvar_flat.flatname, customvar_flat.flatvalue "
	} else {
		query += "SELECT customvar.name, customvar.value "
	}
	query += "FROM " + table + "_customvar " +
		"JOIN " + table + " ON " + table + ".id = " + table + "_customvar." + table + "_id " +
		"JOIN customvar ON customvar.id = " + table + "_customvar.customvar_id "
	if flat {
		query += "JOIN customvar_flat ON customvar_flat.customvar_id = customvar.id "
	}
	query += "WHERE " + table + ".name = ?"

	rows, err := db.Query(db.Rebind(query), name)
	require.NoError(t, err, "querying customvars")
	defer func() { _ = rows.Close() }()

	// copy map to remove items while reading from the database
	expected := make(map[string]sql.NullString)
	if flat {
		for k, v := range c.VarsFlat {
			expected[k] = v
		}
	} else {
		for k, v := range c.Vars {
			expected[k] = toDBString(v)
		}
	}

	for rows.Next() {
		var cvName string
		var cvValue sql.NullString
		err = rows.Scan(&cvName, &cvValue)
		require.NoError(t, err, "scanning query row")

		logger.Debug("custom var from database",
			zap.Bool("flat", flat),
			zap.String("object-type", typeName),
			zap.Any("object-name", name),
			zap.String("custom-var-name", cvName),
			zap.String("custom-var-value", cvValue.String))

		if cvExpected, ok := expected[cvName]; ok {
			assert.Equalf(t, cvExpected, cvValue, "custom var %q", cvName)
			delete(expected, cvName)
		} else if !ok {
			assert.Failf(t, "unexpected custom var", "%q: %q", cvName, cvValue.String)
		}
	}

	for k, v := range expected {
		assert.Failf(t, "missing custom var", "%q: %q", k, v)
	}
}

func makeCustomVarTestData(t *testing.T) []*CustomVarTestData {
	var data []*CustomVarTestData

	// Icinga deduplicates identical custom variables between objects, therefore add a unique identifier to names and
	// values to force it to actually sync new variables instead of just changing the mapping of objects to variables.
	id := utils.UniqueName(t, "customvar")

	// simple string values
	data = append(data, &CustomVarTestData{
		Value: map[string]interface{}{
			id + "-hello": id + " world",
			id + "-foo":   id + " bar",
		},
		Vars: map[string]string{
			id + "-hello": `"` + id + ` world"`,
			id + "-foo":   `"` + id + ` bar"`,
		},
		VarsFlat: map[string]sql.NullString{
			id + "-hello": toDBString(id + " world"),
			id + "-foo":   toDBString(id + " bar"),
		},
	})

	// empty custom vars of type array and dictionaries
	data = append(data, &CustomVarTestData{
		Value: map[string]interface{}{
			id + "-empty-list":        []interface{}{},
			id + "-empty-nested-list": []interface{}{[]interface{}{}},
			id + "-empty-dict":        map[string]interface{}{},
			id + "-empty-nested-dict": map[string]interface{}{
				"some-key": map[string]interface{}{},
			},
		},
		Vars: map[string]string{
			id + "-empty-list":        `[]`,
			id + "-empty-nested-list": `[[]]`,
			id + "-empty-dict":        `{}`,
			id + "-empty-nested-dict": `{"some-key":{}}`,
		},
		VarsFlat: map[string]sql.NullString{
			id + "-empty-list":                 {},
			id + "-empty-nested-list[0]":       {},
			id + "-empty-dict":                 {},
			id + "-empty-nested-dict.some-key": {},
		},
	})

	// complex example
	data = append(data, &CustomVarTestData{
		Value: map[string]interface{}{
			id + "-array": []interface{}{"foo", 23, "bar"},
			id + "-dict": map[string]interface{}{
				"some-key":  "some-value",
				"other-key": "other-value",
			},
			id + "-string": "hello icinga",
			id + "-int":    -1,
			id + "-float":  13.37,
			id + "-true":   true,
			id + "-false":  false,
			id + "-null":   nil,
			id + "-nested-dict": map[string]interface{}{
				"top-level-entry": "good morning",
				"array":           []interface{}{"answer?", 42},
				"dict":            map[string]interface{}{"another-key": "another-value", "yet-another-key": 4711},
			},
			id + "-nested-array": []interface{}{
				[]interface{}{1, 2, 3},
				map[string]interface{}{"contains-a-map": "yes", "really?": true},
				-42,
			},
		},
		Vars: map[string]string{
			id + "-array":        `["foo",23,"bar"]`,
			id + "-dict":         `{"other-key":"other-value","some-key":"some-value"}`,
			id + "-string":       `"hello icinga"`,
			id + "-int":          `-1`,
			id + "-float":        `13.37`,
			id + "-true":         `true`,
			id + "-false":        `false`,
			id + "-null":         `null`,
			id + "-nested-dict":  `{"array":["answer?",42],"dict":{"another-key":"another-value","yet-another-key":4711},"top-level-entry":"good morning"}`,
			id + "-nested-array": `[[1,2,3],{"contains-a-map":"yes","really?":true},-42]`,
		},
		VarsFlat: map[string]sql.NullString{
			id + "-array[0]":                         toDBString(`foo`),
			id + "-array[1]":                         toDBString(`23`),
			id + "-array[2]":                         toDBString(`bar`),
			id + "-dict.some-key":                    toDBString(`some-value`),
			id + "-dict.other-key":                   toDBString(`other-value`),
			id + "-string":                           toDBString(`hello icinga`),
			id + "-int":                              toDBString(`-1`),
			id + "-float":                            toDBString(`13.37`),
			id + "-true":                             toDBString(`true`),
			id + "-false":                            toDBString(`false`),
			id + "-null":                             toDBString(`null`),
			id + "-nested-dict.dict.another-key":     toDBString(`another-value`),
			id + "-nested-dict.dict.yet-another-key": toDBString(`4711`),
			id + "-nested-dict.array[0]":             toDBString(`answer?`),
			id + "-nested-dict.array[1]":             toDBString(`42`),
			id + "-nested-dict.top-level-entry":      toDBString(`good morning`),
			id + "-nested-array[0][0]":               toDBString(`1`),
			id + "-nested-array[0][1]":               toDBString(`2`),
			id + "-nested-array[0][2]":               toDBString(`3`),
			id + "-nested-array[1].contains-a-map":   toDBString(`yes`),
			id + "-nested-array[1].really?":          toDBString(`true`),
			id + "-nested-array[2]":                  toDBString(`-42`),
		},
	})

	// two sets of variables that share keys but have different values
	data = append(data, &CustomVarTestData{
		Value: map[string]interface{}{
			"a": "foo",
			"b": []interface{}{"bar", 42, -13.37},
			"c": map[string]interface{}{"a": true, "b": false, "c": nil},
		},
		Vars: map[string]string{
			"a": `"foo"`,
			"b": `["bar",42,-13.37]`,
			"c": `{"a":true,"b":false,"c":null}`,
		},
		VarsFlat: map[string]sql.NullString{
			"a":    toDBString("foo"),
			"b[0]": toDBString(`bar`),
			"b[1]": toDBString(`42`),
			"b[2]": toDBString(`-13.37`),
			"c.a":  toDBString(`true`),
			"c.b":  toDBString(`false`),
			"c.c":  toDBString(`null`),
		},
	}, &CustomVarTestData{
		Value: map[string]interface{}{
			"a": -13.37,
			"b": []interface{}{true, false, nil},
			"c": map[string]interface{}{"a": "foo", "b": "bar", "c": 42},
		},
		Vars: map[string]string{
			"a": "-13.37",
			"b": `[true,false,null]`,
			"c": `{"a":"foo","b":"bar","c":42}`,
		},
		VarsFlat: map[string]sql.NullString{
			"a":    toDBString(`-13.37`),
			"b[0]": toDBString(`true`),
			"b[1]": toDBString(`false`),
			"b[2]": toDBString(`null`),
			"c.a":  toDBString("foo"),
			"c.b":  toDBString(`bar`),
			"c.c":  toDBString(`42`),
		},
	})

	return data
}

func toDBString(str string) sql.NullString {
	return sql.NullString{String: str, Valid: true}
}
