package main

import (
	"bytes"
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
	"github.com/vbauerster/mpb/v6"
	"github.com/vbauerster/mpb/v6/decor"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"os"
	"strings"
	"time"
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
	Icinga2  struct {
		Env string `yaml:"env"`
	} `yaml:"icinga2"`
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
				err := types[i].snapshot.Get(
					&types[i].total,
					"SELECT COUNT(*) FROM "+types[i].idoTable+
						" xh INNER JOIN icinga_objects o ON o.object_id=xh.object_id",
				)
				if err != nil {
					log.Fatalf("%+v", errors.Wrap(err, "can't count query"))
				}

				log.With("type", types[i].name, "amount", types[i].total).Info("Counted total IDO events")
				return nil
			})
		}

		_ = eg.Wait()
	}

	log.Sync()

	{
		progress := mpb.New()
		for i := range types {
			typ := &types[i]

			typ.bar = progress.AddBar(
				typ.total,
				mpb.BarFillerClearOnComplete(),
				mpb.PrependDecorators(
					decor.Name(typ.name, decor.WC{W: len(typ.name) + 1, C: decor.DidentRight}),
					decor.Percentage(decor.WC{W: 5}),
				),
				mpb.AppendDecorators(decor.EwmaETA(decor.ET_STYLE_GO, 6000000, decor.WC{W: 4})),
			)
		}

		eg, _ := errgroup.WithContext(context.Background())
		for i := range types {
			typ := &types[i]

			if typ.total == 0 {
				typ.bar.SetTotal(typ.bar.Current(), true)
			} else {
				eg.Go(func() error {
					query := "SELECT xh." +
						strings.Join(append(append([]string(nil), typ.idoColumns...), typ.idoIdColumn), ", xh.") +
						" id FROM " + typ.idoTable +
						" xh USE INDEX (PRIMARY) INNER JOIN icinga_objects o ON o.object_id=xh.object_id WHERE " +
						typ.idoIdColumn + " > ? ORDER BY xh." + typ.idoIdColumn + " LIMIT 10000"

					stmt, err := typ.snapshot.Preparex(query)
					if err != nil {
						log.With("query", query).Fatalf("%+v", errors.Wrap(err, "can't prepare query"))
					}
					defer stmt.Close()

					var lastRowsLen int
					var lastQuery string
					var lastStmt *sqlx.Stmt
					start := time.Now()

					defer func() {
						if lastStmt != nil {
							lastStmt.Close()
						}
					}()

				Queries:
					for {
						var rows []ProgressRow
						if err := stmt.Select(&rows, typ.lastId); err != nil {
							log.With("query", query).Fatalf("%+v", errors.Wrap(err, "can't perform query"))
						}

						if len(rows) < 1 {
							break
						}

						if len(rows) != lastRowsLen {
							if lastStmt != nil {
								lastStmt.Close()
							}

							buf := &bytes.Buffer{}
							fmt.Fprintf(
								buf, "SELECT %s FROM %s WHERE %s IN (?", typ.idbIdColumn, typ.idbTable, typ.idbIdColumn,
							)

							for i := 1; i < len(rows); i++ {
								buf.Write([]byte(",?"))
							}

							buf.Write([]byte(")"))
							lastRowsLen = len(rows)
							lastQuery = buf.String()

							var err error
							lastStmt, err = idb.Preparex(lastQuery)

							if err != nil {
								log.With("query", lastQuery).Fatalf("%+v", errors.Wrap(err, "can't prepare query"))
							}
						}

						ids := make([]interface{}, 0, len(rows))
						converted := make([]convertedId, 0, len(rows))

						for _, row := range rows {
							conv := typ.convertId(row, c.Icinga2.Env)
							ids = append(ids, conv)
							converted = append(converted, convertedId{row.Id, conv})
						}

						var present [][]byte
						if err := lastStmt.Select(&present, ids...); err != nil {
							log.With("query", lastQuery).Fatalf("%+v", errors.Wrap(err, "can't perform query"))
						}

						presentSet := map[[20]byte]struct{}{}
						for _, row := range present {
							var key [20]byte
							copy(key[:], row)
							presentSet[key] = struct{}{}
						}

						for _, conv := range converted {
							var key [20]byte
							copy(key[:], conv.idb)

							if _, ok := presentSet[key]; !ok {
								break Queries
							}

							typ.lastId = conv.ido
						}

						prev := start
						now := time.Now()
						start = now

						typ.bar.IncrBy(len(rows))
						typ.bar.DecoratorEwmaUpdate(now.Sub(prev))
					}

					typ.bar.SetTotal(typ.bar.Current(), true)
					return nil
				})
			}
		}

		_ = eg.Wait()
		progress.Wait()
	}

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
