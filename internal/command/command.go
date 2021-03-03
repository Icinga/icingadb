package command

import (
	"fmt"
	"github.com/icinga/icingadb/pkg/config"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/types"
	"github.com/icinga/icingadb/pkg/utils"
	"go.uber.org/zap"
	"path/filepath"
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
		c.Logger.Fatal("can't create database connection pool from config", zap.Error(err))
	}

	return db
}

func (c Command) InstanceId() types.Binary {
	var instanceId types.Binary
	path := filepath.Join(c.Flags.Datadir, "instance-id")

	instanceId, err := utils.CreateOrRead(path, utils.Uuid)
	if err != nil {
		c.Logger.Fatalw(fmt.Sprintf("can't create or read instance-id file %s", path), zap.Error(err))
	}
	instanceId = utils.Checksum([]byte(instanceId))
	c.Logger.Infof("My instance ID is %s", instanceId)

	return instanceId
}

func (c Command) Redis() *icingaredis.Client {
	rc, err := c.Config.Redis.NewClient(c.Logger)
	if err != nil {
		c.Logger.Fatal("can't create Redis client from config", zap.Error(err))
	}

	return rc
}
