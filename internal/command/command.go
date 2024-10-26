package command

import (
	"github.com/icinga/icinga-go-library/config"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icinga-go-library/utils"
	"github.com/icinga/icingadb/internal"
	icingadbconfig "github.com/icinga/icingadb/internal/config"
	"github.com/icinga/icingadb/pkg/icingaredis/telemetry"
	"github.com/pkg/errors"
	"io/fs"
	"os"
	"time"
)

// Command provides factories for creating Redis and Database connections from Config.
type Command struct {
	Flags  icingadbconfig.Flags
	Config icingadbconfig.Config
}

// New parses CLI flags and the YAML configuration and returns a new Command.
// New prints any error during parsing to [os.Stderr] and exits.
func New() *Command {
	var flags icingadbconfig.Flags
	if err := config.ParseFlags(&flags); err != nil {
		if errors.Is(err, config.ErrInvalidArgument) {
			panic(err)
		}

		utils.PrintErrorThenExit(err, 2)
	}

	if flags.Version {
		internal.Version.Print("Icinga DB")
		os.Exit(0)
	}

	// This function supports configuration loading in three scenarios:
	//
	// 1. Load configuration solely from YAML files when no relevant environment variables are set.
	//
	// 2. Combine YAML file and environment variable configurations, allowing environment variables
	//    to supplement or override possible incomplete YAML configurations.
	//
	// 3. Load entirely from environment variables if the default YAML config file is absent and
	//    no specific config path is provided by the user.
	var cfg icingadbconfig.Config
	var configPath string
	if flags.Config != "" {
		configPath = flags.Config
	} else {
		configPath = icingadbconfig.DefaultConfigPath
	}

	if err := config.FromYAMLFile(configPath, &cfg); err != nil {
		if errors.Is(err, config.ErrInvalidArgument) {
			panic(err)
		}

		// Allow continuation with FromEnv by handling:
		//
		// - ErrInvalidConfiguration:
		//   The configuration may be incomplete and will be revalidated in FromEnv.
		//
		// - Non-existent file errors:
		//   If no explicit config path is set, fallback to environment variables is allowed.
		configIsInvalid := errors.Is(err, config.ErrInvalidConfiguration)
		defaultConfigFileDoesNotExist := errors.Is(err, fs.ErrNotExist) &&
			configPath == icingadbconfig.DefaultConfigPath
		if !(configIsInvalid || defaultConfigFileDoesNotExist) {
			utils.PrintErrorThenExit(err, 1)
		}
	}

	// Call FromEnv regardless of the outcome from FromYAMLFile.
	// If no environment variables are set, configuration relies entirely on YAML.
	// Otherwise, environment variables can supplement, override YAML settings, or serve as the sole source.
	// FromEnv also includes validation, ensuring completeness after considering both sources.
	if err := config.FromEnv(&cfg, config.EnvOptions{Prefix: "ICINGADB_"}); err != nil {
		if errors.Is(err, config.ErrInvalidArgument) {
			panic(err)
		}

		utils.PrintErrorThenExit(err, 1)
	}

	return &Command{
		Flags:  flags,
		Config: cfg,
	}
}

// Database creates and returns a new icingadb.DB connection from config.Config.
func (c Command) Database(l *logging.Logger) (*database.DB, error) {
	return database.NewDbFromConfig(&c.Config.Database, l, database.RetryConnectorCallbacks{
		OnRetryableError: func(_ time.Duration, _ uint64, err, _ error) {
			telemetry.UpdateCurrentDbConnErr(err)
		},
		OnSuccess: func(_ time.Duration, _ uint64, _ error) {
			telemetry.UpdateCurrentDbConnErr(nil)
		},
	})
}

// Redis creates and returns a new icingaredis.Client connection from config.Config.
func (c Command) Redis(l *logging.Logger) (*redis.Client, error) {
	return redis.NewClientFromConfig(&c.Config.Redis, l)
}
