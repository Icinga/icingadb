package icingadb_test

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/icinga/icinga-testing/services"
	"github.com/icinga/icinga-testing/utils"
	"github.com/icinga/icinga-testing/utils/eventually"
	localutils "github.com/icinga/icingadb/tests/internal/utils"
	"github.com/icinga/icingadb/tests/internal/value"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"io"
	"reflect"
	"strings"
	"testing"
	"text/template"
	"time"
)

//go:embed object_sync_test.conf
var testSyncConfRaw string
var testSyncConfTemplate = template.Must(template.New("testdata.conf").Parse(testSyncConfRaw))

func TestObjectSync(t *testing.T) {
	logger := it.Logger(t)

	type Data struct {
		GenericPrefixes []string
		Hosts           []Host
		Services        []Service
		Users           []User
	}
	data := &Data{
		// Some name prefixes to loop over in the template to generate multiple instances of objects,
		// for example default-host, some-host, and other-host.
		GenericPrefixes: []string{"default", "some", "other"},

		Hosts:    makeTestSyncHosts(t),
		Services: makeTestSyncServices(t),
		Users:    makeTestUsers(t),
	}

	r := it.RedisServerT(t)
	m := it.MysqlDatabaseT(t)
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
	//logger.Sugar().Infof("config:\n\n%s\n\n", conf.String())
	i.WriteConfig("etc/icinga2/conf.d/testdata.conf", conf.Bytes())
	i.WriteConfig("etc/icinga2/conf.d/api-users.conf", []byte(`
		object ApiUser "root" {
		password = "root"
		permissions = ["*"]
	}
	`))
	i.EnableIcingaDb(r)
	i.Reload()

	// Wait for Icinga 2 to signal a successful dump before starting
	// Icinga DB to ensure that we actually test the initial sync.
	logger.Debug("waiting for icinga2 dump done signal")
	waitForDumpDoneSignal(t, r, 20*time.Second, 100*time.Millisecond)

	// Only after that, start Icinga DB.
	logger.Debug("starting icingadb")
	it.IcingaDbInstanceT(t, r, m)

	db, err := sqlx.Open("mysql", m.DSN())
	require.NoError(t, err, "connecting to mysql shouldn't fail")
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
		// TODO(jb): add tests

		t.Run("User", func(t *testing.T) { t.Parallel(); t.Skip() })
		t.Run("UserGroup", func(t *testing.T) { t.Parallel(); t.Skip() })
		t.Run("Recipient", func(t *testing.T) { t.Parallel(); t.Skip() })
		t.Run("CustomVar", func(t *testing.T) { t.Parallel(); t.Skip() })
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
						"attrs": makeIcinga2ApiAttributes(service),
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
							t.Skip("runtime updates for customvar_flat are not yet implemented") // TODO(jb)
							logger := it.Logger(t)
							eventually.Assert(t, func(t require.TestingT) {
								service.Vars.VerifyCustomVarFlat(t, logger, db, service)
							}, 20*time.Second, 1*time.Second)
						})
					}

					client.DeleteObject(t, "services", *service.HostName+"!"+service.Name, false)

					require.Eventuallyf(t, func() bool {
						var count int
						err := db.Get(&count, "SELECT COUNT(*) FROM service WHERE name = ?", service.Name)
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
								"zone":          service.Zone,
							},
						})
						require.Eventuallyf(t, func() bool {
							var count int
							err := db.Get(&count, "SELECT COUNT(*) FROM service WHERE name = ?", service.Name)
							require.NoError(t, err, "querying service count should not fail")
							return count == 1
						}, 20*time.Second, 1*time.Second, "service with name=%q should exist in database", service.Name)

						client.UpdateObject(t, "services", *service.HostName+"!"+service.Name, map[string]interface{}{
							"attrs": makeIcinga2ApiAttributes(service),
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
								// TODO(jb): needs PR #297
								t.Skip("runtime updates for customvar_flat are not yet implemented")

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
						"attrs": makeIcinga2ApiAttributes(user),
					})

					eventually.Assert(t, func(t require.TestingT) {
						verifyIcingaDbRow(t, db, user)
					}, 20*time.Second, 1*time.Second)

					client.DeleteObject(t, "users", user.Name, false)

					require.Eventuallyf(t, func() bool {
						var count int
						err := db.Get(&count, "SELECT COUNT(*) FROM user WHERE name = ?", user.Name)
						require.NoError(t, err, "querying user count should not fail")
						return count == 0
					}, 20*time.Second, 1*time.Second, "user with name=%q should be removed from database", user.Name)
				})
			}

			t.Run("Update", func(t *testing.T) {
				t.Parallel()

				for _, user := range makeTestUsers(t) {
					user := user

					t.Run(user.VariantInfoString(), func(t *testing.T) {
						t.Parallel()

						client.CreateObject(t, "users", user.Name, map[string]interface{}{
							"attrs": map[string]interface{}{},
						})
						require.Eventuallyf(t, func() bool {
							var count int
							err := db.Get(&count, "SELECT COUNT(*) FROM user WHERE name = ?", user.Name)
							require.NoError(t, err, "querying user count should not fail")
							return count == 1
						}, 20*time.Second, 1*time.Second, "user with name=%q should exist in database", user.Name)

						client.UpdateObject(t, "users", user.Name, map[string]interface{}{
							"attrs": makeIcinga2ApiAttributes(user),
						})

						eventually.Assert(t, func(t require.TestingT) {
							verifyIcingaDbRow(t, db, user)
						}, 20*time.Second, 1*time.Second)

						client.DeleteObject(t, "users", user.Name, false)
					})
				}
			})
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
	CheckCommand          string             `icinga2:"check_command"           icingadb:"checkcommand"`
	MaxCheckAttempts      float64            `icinga2:"max_check_attempts"      icingadb:"max_check_attempts"`
	CheckPeriod           string             `icinga2:"check_period"            icingadb:"check_timeperiod"`
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
	EventCommand          string             `icinga2:"event_command"           icingadb:"eventcommand"`
	Volatile              bool               `icinga2:"volatile"                icingadb:"is_volatile"`
	Zone                  string             `icinga2:"zone"                    icingadb:"zone"`
	CommandEndpoint       string             `icinga2:"command_endpoint"        icingadb:"command_endpoint"`
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
		CheckCommand:          "hostalive",
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
		Vary("CheckCommand", "some-checkcommand", "other-checkcommand").
		Vary("MaxCheckAttempts", 5.0, 7.0).
		Vary("CheckPeriod", "some-timeperiod", "other-timeperiod").
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
		Vary("EventCommand", "some-eventcommand", "other-eventcommand").
		Vary("Volatile", true, false).
		Vary("Zone", "some-zone", "other-zone").
		Vary("CommandEndpoint", "some-endpoint", "other-endpoint").
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

		if hosts[i].Zone == "" {
			hosts[i].Zone = "master"
		}
	}

	return hosts
}

type Service struct {
	Name                  string             `                                  icingadb:"name"`
	DisplayName           string             `icinga2:"display_name"            icingadb:"display_name"`
	HostName              *string            `icinga2:"host_name"               icingadb:"host.name"`
	CheckCommand          string             `icinga2:"check_command"           icingadb:"checkcommand"`
	MaxCheckAttempts      float64            `icinga2:"max_check_attempts"      icingadb:"max_check_attempts"`
	CheckPeriod           string             `icinga2:"check_period"            icingadb:"check_timeperiod"`
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
	EventCommand          string             `icinga2:"event_command"           icingadb:"eventcommand"`
	Volatile              bool               `icinga2:"volatile"                icingadb:"is_volatile"`
	Zone                  string             `icinga2:"zone"                    icingadb:"zone"`
	CommandEndpoint       string             `icinga2:"command_endpoint"        icingadb:"command_endpoint"`
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
		CheckCommand:          "default-checkcommand",
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
		Vary("CheckCommand", "some-checkcommand", "other-checkcommand").
		Vary("MaxCheckAttempts", 5.0, 7.0).
		Vary("CheckPeriod", "some-timeperiod", "other-timeperiod").
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
		Vary("EventCommand", "some-eventcommand", "other-eventcommand").
		Vary("Volatile", true, false).
		Vary("Zone", "some-zone", "other-zone").
		Vary("CommandEndpoint", "some-endpoint", "other-endpoint").
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

		if services[i].Zone == "" {
			services[i].Zone = "master"
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
		if attr := typ.Field(fieldIndex).Tag.Get("icinga2"); attr != "" {
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
// based on the type of obj and its field having icinga2 struct tags.
func makeIcinga2ApiAttributes(obj interface{}) map[string]interface{} {
	attrs := make(map[string]interface{})

	o := reflect.ValueOf(obj)
	typ := o.Type()
	for fieldIndex := 0; fieldIndex < typ.NumField(); fieldIndex++ {
		if attr := typ.Field(fieldIndex).Tag.Get("icinga2"); attr != "" {
			if val := o.Field(fieldIndex).Interface(); val != nil {
				attrs[attr] = value.ToIcinga2Api(val)
			}
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

	type ColumnValueExpected struct {
		Column   string
		Value    interface{}
		Expected interface{}
	}

	joinColumns := func(cs []ColumnValueExpected) string {
		var c []string
		for i := range cs {
			c = append(c, cs[i].Column)
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
		if col := typ.Field(fieldIndex).Tag.Get("icingadb"); col != "" {
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
		joinsQuery += fmt.Sprintf(" LEFT JOIN %s ON %s.id = %s.%s_id", join, join, table, join)
	}

	query := "SELECT " + joinColumns(columns) + " FROM " + table + joinsQuery + " WHERE " + table + ".name = ?"
	rows, err := db.Query(query, name)
	require.NoError(t, err, "mysql query")
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next(), "mysql query should return a row")

	err = rows.Scan(scanSlice(columns)...)
	require.NoError(t, err, "mysql scan")

	for _, col := range columns {
		got := reflect.ValueOf(col.Value).Elem().Interface()
		assert.Equalf(t, col.Expected, got, "%s should match", col.Column)
	}

	require.False(t, rows.Next(), "mysql query should return only one row")
}

// newString allocates a new *string and initializes it. This helper function exists as
// there seems to be no way to achieve this within a single statement.
func newString(s string) *string {
	p := new(string)
	*p = s
	return p
}

type CustomVarTestData struct {
	Value    interface{}       // Value to put into config or API
	Vars     map[string]string // Expected values in customvar table
	VarsFlat map[string]string // Expected values in customvar_flat table
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

	rows, err := db.Query(query, name)
	defer func() { _ = rows.Close() }()
	require.NoError(t, err, "querying customvars")

	expectedSrc := c.Vars
	if flat {
		expectedSrc = c.VarsFlat
	}

	// copy map to remove items while reading from the database
	expected := make(map[string]string)
	for k, v := range expectedSrc {
		expected[k] = v
	}

	for rows.Next() {
		var cvName, cvValue string
		err := rows.Scan(&cvName, &cvValue)
		require.NoError(t, err, "scanning query row")

		logger.Debug("custom var from database",
			zap.Bool("flat", flat),
			zap.String("object-type", typeName),
			zap.Any("object-name", name),
			zap.String("custom-var-name", cvName),
			zap.String("custom-var-value", cvValue))

		if cvExpected, ok := expected[cvName]; ok {
			assert.Equalf(t, cvExpected, cvValue, "custom var %q", cvName)
			delete(expected, cvName)
		} else if !ok {
			assert.Failf(t, "unexpected custom var", "%q: %q", cvName, cvValue)
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
		VarsFlat: map[string]string{
			id + "-hello": id + " world",
			id + "-foo":   id + " bar",
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
		VarsFlat: map[string]string{
			id + "-array[0]":                         `foo`,
			id + "-array[1]":                         `23`,
			id + "-array[2]":                         `bar`,
			id + "-dict.some-key":                    `some-value`,
			id + "-dict.other-key":                   `other-value`,
			id + "-string":                           `hello icinga`,
			id + "-int":                              `-1`,
			id + "-float":                            `13.37`,
			id + "-true":                             `true`,
			id + "-false":                            `false`,
			id + "-null":                             `null`,
			id + "-nested-dict.dict.another-key":     `another-value`,
			id + "-nested-dict.dict.yet-another-key": `4711`,
			id + "-nested-dict.array[0]":             `answer?`,
			id + "-nested-dict.array[1]":             `42`,
			id + "-nested-dict.top-level-entry":      `good morning`,
			id + "-nested-array[0][0]":               `1`,
			id + "-nested-array[0][1]":               `2`,
			id + "-nested-array[0][2]":               `3`,
			id + "-nested-array[1].contains-a-map":   `yes`,
			id + "-nested-array[1].really?":          `true`,
			id + "-nested-array[2]":                  `-42`,
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
		VarsFlat: map[string]string{
			"a":    "foo",
			"b[0]": `bar`,
			"b[1]": `42`,
			"b[2]": `-13.37`,
			"c.a":  `true`,
			"c.b":  `false`,
			"c.c":  `null`,
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
		VarsFlat: map[string]string{
			"a":    `-13.37`,
			"b[0]": `true`,
			"b[1]": `false`,
			"b[2]": `null`,
			"c.a":  "foo",
			"c.b":  `bar`,
			"c.c":  `42`,
		},
	})

	return data
}
