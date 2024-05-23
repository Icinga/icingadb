package command

import (
	"fmt"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/config"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/logging"
	goflags "github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"os"
)

// Command provides factories for creating Redis and Database connections from Config.
type Command struct {
	Flags  *config.Flags
	Config *config.Config
}

// New creates and returns a new Command, parses CLI flags and YAML the config, and initializes the logger.
func New() *Command {
	flags, err := config.ParseFlags()
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

	var cfg *config.Config
	if flags.ConfigFromEnv {
		cfg, err = config.FromEnv()
	} else {
		cfg, err = config.FromYAMLFile(flags.Config)
	}

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
func (c Command) Database(l *logging.Logger) (*icingadb.DB, error) {
	return c.Config.Database.Open(l)
}

// Redis creates and returns a new icingaredis.Client connection from config.Config.
func (c Command) Redis(l *logging.Logger) (*icingaredis.Client, error) {
	return c.Config.Redis.NewClient(l)
}
