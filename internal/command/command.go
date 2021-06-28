package command

import (
	"github.com/icinga/icingadb/internal/logging"
	"github.com/icinga/icingadb/pkg/config"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Command struct {
	Flags  *config.Flags
	Config *config.Config
	Logger *zap.SugaredLogger
}

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

func (c Command) Database(logging2 *logging.Logging) *icingadb.DB {
	db, err := c.Config.Database.Open(logging2.GetLogger("database"))
	if err != nil {
		c.Logger.Fatalf("%+v", errors.Wrap(err, "can't create database connection pool from config"))
	}

	return db
}

func (c Command) Redis(logging2 *logging.Logging) *icingaredis.Client {
	rc, err := c.Config.Redis.NewClient(logging2.GetLogger("redis"))
	if err != nil {
		c.Logger.Fatalf("%+v", errors.Wrap(err, "can't create Redis client from config"))
	}

	return rc
}

func (c Command) Logging() *logging.Logging {
	rc, err := c.Config.Logging.NewLogger()
	if err != nil {
		c.Logger.Fatalf("%+v", errors.Wrap(err, "can't create Logger from config"))
	}

	return rc
}
