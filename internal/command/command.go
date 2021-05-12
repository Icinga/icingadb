package command

import (
	"github.com/icinga/icingadb/pkg/config"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/utils"
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

	logger, err := zap.NewDevelopment()
	if err != nil {
		utils.Fatal(err)
	}
	sugar := logger.Sugar()

	return &Command{
		Flags:  flags,
		Config: cfg,
		Logger: sugar,
	}
}

func (c Command) Database() *icingadb.DB {
	db, err := c.Config.Database.Open(c.Logger)
	if err != nil {
		c.Logger.Fatalw("can't create database connection pool from config", zap.Error(err))
	}

	return db
}

func (c Command) Redis() *icingaredis.Client {
	rc, err := c.Config.Redis.NewClient(c.Logger)
	if err != nil {
		c.Logger.Fatalw("can't create Redis client from config", zap.Error(err))
	}

	return rc
}
