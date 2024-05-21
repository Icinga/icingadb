package command

import (
	"fmt"
	"github.com/icinga/icingadb/internal"
	icingadbconfig "github.com/icinga/icingadb/internal/config"
	"github.com/icinga/icingadb/pkg/config"
	"github.com/icinga/icingadb/pkg/database"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/icingaredis/telemetry"
	"github.com/icinga/icingadb/pkg/logging"
	goflags "github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"os"
	"time"
)

// Command provides factories for creating Redis and Database connections from Config.
type Command struct {
	Flags  *icingadbconfig.Flags
	Config *icingadbconfig.Config
}

// New creates and returns a new Command, parses CLI flags and YAML the config, and initializes the logger.
func New() *Command {
	flags, err := config.ParseFlags[icingadbconfig.Flags]()
	if err != nil {
		var cliErr *goflags.Error
		if errors.As(err, &cliErr) && cliErr.Type == goflags.ErrHelp {
			os.Exit(0)
		}

		os.Exit(2)
	}

	if flags.Version {
		internal.Version.Print()
		os.Exit(0)
	}

	cfg, err := config.FromYAMLFile[icingadbconfig.Config](flags.Config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
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
func (c Command) Redis(l *logging.Logger) (*icingaredis.Client, error) {
	return c.Config.Redis.NewClient(l)
}
