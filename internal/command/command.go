package command

import (
	"fmt"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/internal/logging"
	"github.com/icinga/icingadb/pkg/config"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/utils"
	goflags "github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"go.uber.org/zap"
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
		fmt.Println("Icinga DB version:", internal.Version)
		os.Exit(0)
	}

	cfg, err := config.FromYAMLFile(flags.Config)
	if err != nil {
		utils.Fatal(err)
	}

	return &Command{
		Flags:  flags,
		Config: cfg,
	}
}

// Database creates and returns a new icingadb.DB connection from config.Config.
func (c Command) Database(l *zap.SugaredLogger) (*icingadb.DB, error) {
	return c.Config.Database.Open(l)
}

// Redis creates and returns a new icingaredis.Client connection from config.Config.
func (c Command) Redis(l *zap.SugaredLogger) (*icingaredis.Client, error) {
	return c.Config.Redis.NewClient(l)
}

func (c Command) Logging() *logging.Logging {

	return c.Config.Logging.NewLogging()
}
