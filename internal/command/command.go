package command

import (
	"github.com/icinga/icingadb/pkg/config"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/icingaredis"
	"github.com/icinga/icingadb/pkg/utils"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/client/pkg/v3/logutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
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

	loggerCfg := logutil.DefaultZapLoggerConfig
	// Disable zap's automatic stack trace capturing, as we call errors.Wrap() before logging with "%+v".
	loggerCfg.DisableStacktrace = true

	enc := zapcore.NewJSONEncoder(loggerCfg.EncoderConfig)

	// To write log output to the local systemd journal
	writer, err := logutil.NewJournalWriter(os.Stderr)

	if err != nil {
		utils.Fatal(errors.Wrap(err, "can't create logger"))
	}

	level := zap.NewAtomicLevelAt(logutil.ConvertToZapLevel(logutil.DefaultLogLevel))

	syncer := zapcore.AddSync(writer)
	core := zapcore.NewCore(enc, syncer, level)

	logger := zap.New(core, zap.AddCaller(), zap.ErrorOutput(syncer))

	defer logger.Sync()

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
		c.Logger.Fatalf("%+v", errors.Wrap(err, "can't create database connection pool from config"))
	}

	return db
}

func (c Command) Redis() *icingaredis.Client {
	rc, err := c.Config.Redis.NewClient(c.Logger)
	if err != nil {
		c.Logger.Fatalf("%+v", errors.Wrap(err, "can't create Redis client from config"))
	}

	return rc
}
