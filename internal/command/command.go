package command

import (
	"github.com/icinga/icingadb/pkg/cleanup"
	"github.com/icinga/icingadb/pkg/config"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// Command provides factories for creating Redis and Database connections from Config.
type Command struct {
	Flags  *config.Flags
	Config *config.Config
	Logger *zap.SugaredLogger
}

// New creates and returns a new Command, parses CLI flags and YAML the config, and initializes the logger.
func New() *Command {
	flags, err := config.ParseFlags()
	if err != nil {
		utils.Fatal(err)
	}

	cfg, err := config.FromYAMLFile(flags.Config)
	if err != nil {
		utils.Fatal(err)
	}

	loggerCfg := zap.NewDevelopmentConfig()
	// Disable zap's automatic stack trace capturing, as we call errors.Wrap() before logging with "%+v".
	loggerCfg.DisableStacktrace = true
	logger, err := loggerCfg.Build()
	if err != nil {
		utils.Fatal(errors.Wrap(err, "can't create logger"))
	}
	sugar := logger.Sugar()

	return &Command{
		Flags:  flags,
		Config: cfg,
		Logger: sugar,
	}
}

// Database creates and returns a new icingadb.DB connection from config.Config.
func (c Command) Database() *icingadb.DB {
	db, err := c.Config.Database.Open(c.Logger)
	if err != nil {
		c.Logger.Fatalf("%+v", errors.Wrap(err, "can't create database connection pool from config"))
	}

	return db
}

// Redis creates and returns a new icingaredis.Client connection from config.Config.
func (c Command) Redis() *icingaredis.Client {
	rc, err := c.Config.Redis.NewClient(c.Logger)
	if err != nil {
		c.Logger.Fatalf("%+v", errors.Wrap(err, "can't create Redis client from config"))
	}

	return rc
}

func (c Command) Cleanup(db *icingadb.DB) *cleanup.Cleanup {
	cu := c.Config.Cleanup.NewCleanup(db, c.Logger)

	return cu
}
