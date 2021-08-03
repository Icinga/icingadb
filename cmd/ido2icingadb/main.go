package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/goccy/go-yaml"
	"github.com/icinga/icingadb/cmd/internal"
	"github.com/icinga/icingadb/pkg/config"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/jessevdk/go-flags"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"os"
)

// Flags defines the CLI flags.
type Flags struct {
	// Config is the path to the config file.
	Config string `short:"c" long:"config" description:"path to config file" required:"true"`
}

// Config defines the YAML config structure.
type Config struct {
	IDO      config.Database `yaml:"ido"`
	IcingaDB config.Database `yaml:"icingadb"`
}

func main() {
	os.Exit(run())
}

func run() int {
	f := &Flags{}
	if _, err := flags.NewParser(f, flags.Default).Parse(); err != nil {
		return 2
	}

	cf, err := os.Open(f.Config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't open config file: %s\n", err.Error())
		return 2
	}

	c := &Config{}

	{
		err := yaml.NewDecoder(cf).Decode(c)
		cf.Close()

		if err != nil {
			fmt.Fprintf(os.Stderr, "can't parse config file: %s\n", err.Error())
			return 2
		}
	}

	logger, _ := zap.NewDevelopmentConfig().Build()

	log := logger.Sugar()
	defer log.Sync()

	log.Info("Starting IDO to Icinga DB history migration")

	log.Info("Connecting to databases")
	var ido, idb *icingadb.DB

	{
		eg, _ := errgroup.WithContext(context.Background())

		eg.Go(func() error {
			ido = connect(log, "IDO", &c.IDO)
			return nil
		})

		eg.Go(func() error {
			idb = connect(log, "Icinga DB", &c.IcingaDB)
			return nil
		})

		_ = eg.Wait()
	}

	var types = []struct {
		name     string
		idoTable string
		snapshot *sqlx.Tx
	}{
		{"acknowledgement", "icinga_acknowledgements", nil}, {"comment", "icinga_commenthistory", nil},
		{"downtime", "icinga_downtimehistory", nil}, {"flapping", "icinga_flappinghistory", nil},
		{"notification", "icinga_notifications", nil}, {"state", "icinga_statehistory", nil},
	}

	{
		eg, _ := errgroup.WithContext(context.Background())
		for i := range types {
			i := i

			eg.Go(func() error {
				tx, err := ido.BeginTxx(context.Background(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
				if err != nil {
					log.Fatalf("%+v", errors.Wrap(err, "can't begin snapshot transaction"))
				}

				types[i].snapshot = tx
				return nil
			})
		}

		_ = eg.Wait()
	}

	log.Info("Computing progress")

	{
		eg, _ := errgroup.WithContext(context.Background())
		for i := range types {
			i := i

			eg.Go(func() error {
				var count uint64

				err := types[i].snapshot.Get(
					&count,
					"SELECT COUNT(*) FROM "+types[i].idoTable+
						" xh INNER JOIN icinga_objects o ON o.object_id=xh.object_id",
				)
				if err != nil {
					log.Fatalf("%+v", errors.Wrap(err, "can't count query"))
				}

				log.With("type", types[i].name, "amount", count).Info("Counted total IDO events")
				return nil
			})
		}

		_ = eg.Wait()
	}

	// TODO
	_ = idb

	return internal.ExitSuccess
}

func connect(log *zap.SugaredLogger, which string, cfg *config.Database) *icingadb.DB {
	db, err := cfg.Open(log)
	if err != nil {
		log.With("backend", which).Fatalf("%+v", errors.Wrap(err, "can't connect to database"))
	}

	if err := db.Ping(); err != nil {
		log.With("backend", which).Fatalf("%+v", errors.Wrap(err, "can't connect to database"))
	}

	return db
}
