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

	var cfg icingadbconfig.Config
	if err := config.FromYAMLFile(flags.Config, &cfg); err != nil {
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
