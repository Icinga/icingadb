package config

import (
	"github.com/caarlos0/env/v11"
	"github.com/creasty/defaults"
	"github.com/icinga/icinga-go-library/config"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/notifications/source"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icinga-go-library/testutils"
	"github.com/icinga/icingadb/pkg/icingadb/history"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"os"
	"testing"
)

// testFlags is a struct that implements the Flags interface.
// It holds information about the configuration file path and whether it was explicitly set.
type testFlags struct {
	configPath         string // The path to the configuration file.
	explicitConfigPath bool   // Indicates if the config path was explicitly set.
}

// GetConfigPath returns the path to the configuration file.
func (f testFlags) GetConfigPath() string {
	return f.configPath
}

// IsExplicitConfigPath indicates whether the configuration file path was explicitly set.
func (f testFlags) IsExplicitConfigPath() bool {
	return f.explicitConfigPath
}

func TestConfig(t *testing.T) {
	const yamlConfig = `
database:
  host: 192.0.2.1
  database: icingadb
  user: icingadb
  password: icingadb

redis:
  host: 2001:db8::1
`
	loadTests := []testutils.TestCase[config.Validator, testutils.ConfigTestData]{
		{
			Name: "Load from YAML only",
			Data: testutils.ConfigTestData{
				Yaml: yamlConfig + `
logging:
  options:
    database: debug
    redis: debug

retention:
  options:
    comment: 31
    downtime: 365
`,
			},
			Expected: &Config{
				Database: database.Config{
					Host:     "192.0.2.1",
					Database: "icingadb",
					User:     "icingadb",
					Password: "icingadb",
				},
				Redis: redis.Config{
					Host: "2001:db8::1",
				},
				Logging: logging.Config{
					Options: logging.Options{
						"database": zapcore.DebugLevel,
						"redis":    zapcore.DebugLevel,
					},
				},
				Retention: RetentionConfig{
					Options: history.RetentionOptions{
						"comment":  31,
						"downtime": 365,
					},
				},
			},
		},
		{
			Name: "Load from Env only",
			Data: testutils.ConfigTestData{
				Env: map[string]string{
					"ICINGADB_DATABASE_HOST":     "192.0.2.1",
					"ICINGADB_DATABASE_DATABASE": "icingadb",
					"ICINGADB_DATABASE_USER":     "icingadb",
					"ICINGADB_DATABASE_PASSWORD": "icingadb",
					"ICINGADB_REDIS_HOST":        "2001:db8::1",
					"ICINGADB_LOGGING_OPTIONS":   "database:debug,redis:debug",
					"ICINGADB_RETENTION_OPTIONS": "comment:31,downtime:365",
				},
			},
			Expected: &Config{
				Database: database.Config{
					Host:     "192.0.2.1",
					Database: "icingadb",
					User:     "icingadb",
					Password: "icingadb",
				},
				Redis: redis.Config{
					Host: "2001:db8::1",
				},
				Logging: logging.Config{
					Options: logging.Options{
						"database": zapcore.DebugLevel,
						"redis":    zapcore.DebugLevel,
					},
				},
				Retention: RetentionConfig{
					Options: history.RetentionOptions{
						"comment":  31,
						"downtime": 365,
					},
				},
			},
		},
		{
			Name: "YAML and Env; Env overrides",
			Data: testutils.ConfigTestData{
				Yaml: yamlConfig,
				Env: map[string]string{
					"ICINGADB_DATABASE_HOST": "192.168.0.1",
					"ICINGADB_REDIS_HOST":    "localhost",
				},
			},
			Expected: &Config{
				Database: database.Config{
					Host:     "192.168.0.1",
					Database: "icingadb",
					User:     "icingadb",
					Password: "icingadb",
				},
				Redis: redis.Config{
					Host: "localhost",
				},
			},
		},
		{
			Name: "YAML and Env; Env supplements",
			Data: testutils.ConfigTestData{
				Yaml: yamlConfig,
				Env: map[string]string{
					"ICINGADB_REDIS_USERNAME": "icingadb",
					"ICINGADB_REDIS_PASSWORD": "icingadb",
				}},
			Expected: &Config{
				Database: database.Config{
					Host:     "192.0.2.1",
					Database: "icingadb",
					User:     "icingadb",
					Password: "icingadb",
				},
				Redis: redis.Config{
					Host:     "2001:db8::1",
					Username: "icingadb",
					Password: "icingadb",
				},
			},
		},
		{
			Name: "YAML and Env; Env overrides defaults",
			Data: testutils.ConfigTestData{
				Yaml: yamlConfig,
				Env: map[string]string{
					"ICINGADB_DATABASE_PORT": "3307",
				}},
			Expected: &Config{
				Database: database.Config{
					Host:     "192.0.2.1",
					Port:     3307,
					Database: "icingadb",
					User:     "icingadb",
					Password: "icingadb",
				},
				Redis: redis.Config{
					Host: "2001:db8::1",
				},
			},
		},
		{
			Name: "Custom Notifications Config from YAML",
			Data: testutils.ConfigTestData{
				Yaml: yamlConfig + `
notifications:
  url: http://example.com/
  username: icingadb
  password: insecure
`,
			},
			Expected: &Config{
				Database: database.Config{
					Host:     "192.0.2.1",
					Database: "icingadb",
					User:     "icingadb",
					Password: "icingadb",
				},
				Redis: redis.Config{
					Host: "2001:db8::1",
				},
				Notifications: NotificationsConfig{
					Config: source.Config{
						Url:      "http://example.com/",
						Username: "icingadb",
						Password: "insecure",
					},
					SynchronizeWithDatabase: true,
				},
			},
		},
		{
			Name: "Unknown YAML field",
			Data: testutils.ConfigTestData{
				Yaml: `unknown: unknown`,
			},
			Error: testutils.ErrorContains(`unknown field "unknown"`),
		},
	}

	for _, tc := range loadTests {
		t.Run(tc.Name, tc.F(func(data testutils.ConfigTestData) (config.Validator, error) {
			if tc.Error == nil {
				// Set defaults for the expected configuration if no error is expected.
				require.NoError(t, defaults.Set(tc.Expected), "setting defaults")
			}

			actual := new(Config)

			var err error
			if data.Yaml != "" {
				testutils.WithYAMLFile(t, data.Yaml, func(file *os.File) {
					err = config.Load(actual, config.LoadOptions{
						Flags:      testFlags{configPath: file.Name(), explicitConfigPath: true},
						EnvOptions: config.EnvOptions{Prefix: "ICINGADB_", Environment: data.Env},
					})
				})
			} else {
				err = config.Load(actual, config.LoadOptions{
					Flags:      testFlags{},
					EnvOptions: config.EnvOptions{Prefix: "ICINGADB_", Environment: data.Env},
				})
			}

			return actual, err
		}))
	}
}

func TestNotificationsConfig_StaticConfig(t *testing.T) {
	cfg := NotificationsConfig{
		Config: source.Config{
			Url:              "https://example.com:5680",
			Username:         "icingadb",
			Password:         "icingadb",
			TlsOptions:       config.TLS{Insecure: true},
			DefaultRelations: []string{"$.foo", "$.bar"},
		},
	}

	staticConf := cfg.StaticConfig()
	require.Equal(t, map[string]string{
		"ICINGADB_NOTIFICATIONS_URL":               "https://example.com:5680",
		"ICINGADB_NOTIFICATIONS_USERNAME":          "icingadb",
		"ICINGADB_NOTIFICATIONS_PASSWORD":          "REDACTED",
		"ICINGADB_NOTIFICATIONS_INSECURE":          "true",
		"ICINGADB_NOTIFICATIONS_DEFAULT_RELATIONS": "$.foo,$.bar",
	}, staticConf)

	var cmpCfg Config
	require.NoError(t, env.ParseWithOptions(&cmpCfg, env.Options{Prefix: "ICINGADB_", Environment: staticConf}))
	cmpCfg.Notifications.Config.Password = "icingadb" // Reset, as redacted in output
	require.Equal(t, cfg, cmpCfg.Notifications)
}
