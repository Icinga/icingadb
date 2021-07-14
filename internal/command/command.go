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
		Config: cfg}
}

// Database creates and returns a new icingadb.DB connection from config.Config.
func (c Command) Database(logging2 *logging.Logging) *icingadb.DB {
	db, err := c.Config.Database.Open(logging2.GetChildLogger("database"))
	if err != nil {
		logging2.Fatalf("%+v", errors.Wrap(err, "can't create database connection pool from config"))
	}

	return db
}

// Redis creates and returns a new icingaredis.Client connection from config.Config.
func (c Command) Redis(logging2 *logging.Logging) *icingaredis.Client {
	rc, err := c.Config.Redis.NewClient(logging2.GetChildLogger("redis"))
	if err != nil {
		logging2.Fatalf("%+v", errors.Wrap(err, "can't create Redis client from config"))
	}

	return rc
}

func (c Command) Logging() *logging.Logging {
	rc, err := c.Config.Logging.NewLogger()
	if err != nil {
		rc.Fatalf("%+v", errors.Wrap(err, "can't create Logger from config"))
	}

	return rc
}
