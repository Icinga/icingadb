package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"database/sql"
	"fmt"
	"github.com/goccy/go-yaml"
	"github.com/icinga/icingadb/cmd/internal"
	"github.com/icinga/icingadb/pkg/config"
	"github.com/icinga/icingadb/pkg/icingadb"
	icingadbTypes "github.com/icinga/icingadb/pkg/types"
	"github.com/jessevdk/go-flags"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/reflectx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/vbauerster/mpb/v6"
	"golang.org/x/sync/errgroup"
	"os"
	"path"
	"reflect"
	"strings"
	"time"
)

// Flags defines the CLI flags.
type Flags struct {
	// Config is the path to the config file.
	Config string `short:"c" long:"config" description:"path to config file" required:"true"`
	// Cache is a (not necessarily yet existing) directory for caching.
	Cache string `short:"t" long:"cache" description:"path for caching" required:"true"`
}

// Config defines the YAML config structure.
type Config struct {
	IDO      config.Database `yaml:"ido"`
	IcingaDB config.Database `yaml:"icingadb"`
	Icinga2  struct {
		Env      string `yaml:"env"`
		Endpoint string `yaml:"endpoint"`
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

	c, ex := parseConfig(f)
	if c == nil {
		return ex
	}

	defer log.Sync()

	log.Info("Starting IDO to Icinga DB history migration")

	ido, idb := connectAll(c)
	startIdoTx(ido)
	mkCache(f, idb.Mapper)

	log.Info("Computing progress")

	countIdoHistory()
	log.Sync()
	computeProgress(c, idb)

	log.Info("Filling cache")
	fillCache()

	log.Info("Actually migrating")
	migrate(c, idb)

	return internal.ExitSuccess
}

func parseConfig(f *Flags) (*Config, int) {
	cf, err := os.Open(f.Config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't open config file: %s\n", err.Error())
		return nil, 2
	}
	defer cf.Close()

	c := &Config{}
	if err := yaml.NewDecoder(cf).Decode(c); err != nil {
		fmt.Fprintf(os.Stderr, "can't parse config file: %s\n", err.Error())
		return nil, 2
	}

	return c, -1
}

func mkCache(f *Flags, mapper *reflectx.Mapper) {
	log.Info("Preparing cache")

	if err := os.MkdirAll(f.Cache, 0700); err != nil {
		log.With("dir", f.Cache).Fatalf("%+v", errors.Wrap(err, "can't create directory"))
	}

	types.forEach(func(ht *historyType) {
		if ht.cacheSchema == nil {
			return
		}

		file := path.Join(f.Cache, ht.name+".sqlite3")
		var err error

		ht.cache, err = sqlx.Open("sqlite3", "file:"+file)
		if err != nil {
			log.With("file", file).Fatalf("%+v", errors.Wrap(err, "can't open SQLite database"))
		}

		ht.cache.Mapper = mapper

		for _, ddl := range ht.cacheSchema {
			if _, err := ht.cache.Exec(ddl); err != nil {
				log.With("file", file, "ddl", ddl).
					Fatalf("%+v", errors.Wrap(err, "can't import schema into SQLite database"))
			}
		}
	})
}

func connectAll(c *Config) (ido, idb *icingadb.DB) {
	log.Info("Connecting to databases")
	eg, _ := errgroup.WithContext(context.Background())

	eg.Go(func() error {
		ido = connect("IDO", &c.IDO)
		return nil
	})

	eg.Go(func() error {
		idb = connect("Icinga DB", &c.IcingaDB)
		return nil
	})

	_ = eg.Wait()
	return
}

func connect(which string, cfg *config.Database) *icingadb.DB {
	db, err := cfg.Open(log)
	if err != nil {
		log.With("backend", which).Fatalf("%+v", errors.Wrap(err, "can't connect to database"))
	}

	if err := db.Ping(); err != nil {
		log.With("backend", which).Fatalf("%+v", errors.Wrap(err, "can't connect to database"))
	}

	return db
}

func startIdoTx(ido *icingadb.DB) {
	types.forEach(func(ht *historyType) {
		tx, err := ido.BeginTxx(context.Background(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
		if err != nil {
			log.Fatalf("%+v", errors.Wrap(err, "can't begin snapshot transaction"))
		}

		ht.snapshot = tx
	})
}

func countIdoHistory() {
	types.forEach(func(ht *historyType) {
		err := ht.snapshot.Get(
			&ht.total,
			"SELECT COUNT(*) FROM "+ht.idoTable+" xh INNER JOIN icinga_objects o ON o.object_id=xh.object_id",
		)
		if err != nil {
			log.Fatalf("%+v", errors.Wrap(err, "can't count query"))
		}

		log.With("type", ht.name, "amount", ht.total).Info("Counted total IDO events")
	})
}

func computeProgress(c *Config, idb *icingadb.DB) {
	progress := mpb.New()
	for i := range types {
		types[i].setupBar(progress)
	}

	types.forEach(func(ht *historyType) {
		if ht.total == 0 {
			ht.bar.SetTotal(ht.bar.Current(), true)
			return
		}

		query := "SELECT xh." +
			strings.Join(append(append([]string(nil), ht.idoColumns...), ht.idoIdColumn), ", xh.") + " id FROM " +
			ht.idoTable + " xh USE INDEX (PRIMARY) INNER JOIN icinga_objects o ON o.object_id=xh.object_id WHERE " +
			ht.idoIdColumn + " > ? ORDER BY xh." + ht.idoIdColumn + " LIMIT ?"

		var lastRowsLen int
		var lastQuery string
		var lastStmt *sqlx.Stmt
		inc := barIncrementer{ht.bar, time.Now()}

		defer func() {
			if lastStmt != nil {
				lastStmt.Close()
			}
		}()

		sliceIdoHistory(ht.snapshot, query, nil, 0, func(rows []ProgressRow) (checkpoint interface{}) {
			if len(rows) != lastRowsLen {
				if lastStmt != nil {
					lastStmt.Close()
				}

				buf := &bytes.Buffer{}
				fmt.Fprintf(buf, "SELECT %s FROM %s WHERE %s IN (?", ht.idbIdColumn, ht.idbTable, ht.idbIdColumn)

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
				conv := ht.convertId(row, c.Icinga2.Env)
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
					return nil
				}

				ht.lastId = conv.ido
			}

			inc.inc(len(rows))
			return ht.lastId
		})

		ht.bar.SetTotal(ht.bar.Current(), true)
	})

	progress.Wait()
}

func fillCache() {
	progress := mpb.New()
	for i := range types {
		if types[i].cacheFiller != nil {
			types[i].setupBar(progress)
		}
	}

	types.forEach(func(ht *historyType) {
		if ht.cacheFiller != nil {
			ht.cacheFiller(ht)
		}
	})

	progress.Wait()
}

var tAny = reflect.TypeOf((*interface{})(nil)). // *interface{}
						Elem() // interface{}

func migrate(c *Config, idb *icingadb.DB) {
	envId := sha1.Sum([]byte(c.Icinga2.Env))
	endpointId := sha1.Sum([]byte(c.Icinga2.Endpoint))

	progress := mpb.New()
	for i := range types {
		types[i].setupBar(progress)
	}

	types.forEach(func(ht *historyType) {
		vConvertRows := reflect.ValueOf(ht.convertRows) // TODO: make historyType#convertRows generic[T] one nice day

		tRows := vConvertRows.Type(). // func(env string, envId, endpointId Binary, selectCache func(dest interface{}, query string, args ...interface{}), ido *sqlx.Tx, idoRows []T) (icingaDbUpdates, icingaDbInserts [][]interface{}, checkpoint interface{})
						In(5) // []T

		var lastQuery string
		var lastStmt *sqlx.Stmt

		defer func() {
			if lastStmt != nil {
				_ = lastStmt.Close()
			}
		}()

		vConvertRowsArgs := [6]reflect.Value{
			reflect.ValueOf(c.Icinga2.Env), reflect.ValueOf(icingadbTypes.Binary(envId[:])),
			reflect.ValueOf(icingadbTypes.Binary(endpointId[:])),
			reflect.ValueOf(func(dest interface{}, query string, args ...interface{}) {
				if query != lastQuery {
					if lastStmt != nil {
						_ = lastStmt.Close()
					}

					var err error

					lastStmt, err = ht.cache.Preparex(query)
					if err != nil {
						log.With("backend", "cache", "query", query).
							Fatalf("%+v", errors.Wrap(err, "can't prepare query"))
					}

					lastQuery = query
				}

				if err := lastStmt.Select(dest, args...); err != nil {
					log.With("backend", "cache", "query", query, "args", args).
						Fatalf("%+v", errors.Wrap(err, "can't perform query"))
				}
			}),
			reflect.ValueOf(ht.snapshot),
		}

		var args []interface{}
		if ht.cacheLimitQuery != "" {
			var limit uint64
			cacheGet(ht.cache, &limit, ht.cacheLimitQuery)
			args = append(args, limit)
		}

		icingaDbInserts := map[reflect.Type]string{}
		icingaDbUpdates := map[reflect.Type]string{}
		inc := barIncrementer{ht.bar, time.Now()}

		sliceIdoHistory(
			ht.snapshot, ht.migrationQuery, args, ht.lastId,
			reflect.MakeFunc(reflect.FuncOf([]reflect.Type{tRows}, []reflect.Type{tAny}, false),
				func(args []reflect.Value) []reflect.Value {
					vConvertRowsArgs[5] = args[0]
					res := vConvertRows.Call(vConvertRowsArgs[:])

					tx, err := idb.Beginx()
					if err != nil {
						log.With("backend", "Icinga DB").Fatalf("%+v", errors.Wrap(err, "can't begin transaction"))
					}

					for _, table := range res[0].Interface().([][]interface{}) {
						if len(table) < 1 {
							continue
						}

						tRow := reflect.TypeOf(table[0])

						update, ok := icingaDbUpdates[tRow]
						if !ok {
							update, _ = idb.BuildUpdateStmt(table[0])
							icingaDbUpdates[tRow] = update
						}

						stmt, err := tx.PrepareNamed(update)
						if err != nil {
							log.With("backend", "Icinga DB", "dml", update).
								Fatalf("%+v", errors.Wrap(err, "can't prepare DML"))
						}

						for _, row := range table {
							if _, err := stmt.Exec(row); err != nil {
								log.With("backend", "Icinga DB", "dml", update, "args", row).
									Fatalf("%+v", errors.Wrap(err, "can't perform DML"))
							}
						}

						_ = stmt.Close()
					}

					for _, table := range res[1].Interface().([][]interface{}) {
						if len(table) < 1 {
							continue
						}

						tRow := reflect.TypeOf(table[0])

						insert, ok := icingaDbInserts[tRow]
						if !ok {
							insert, _ = idb.BuildInsertStmt(table[0])
							insert = "REPLACE " + strings.TrimPrefix(insert, "INSERT ")
							icingaDbInserts[tRow] = insert
						}

						for len(table) > 0 {
							slice := table
							if len(slice) > 1000 {
								slice = slice[:1000]
								table = table[1000:]
							} else {
								table = nil
							}

							if _, err := tx.NamedExec(insert, slice); err != nil {
								log.With("backend", "Icinga DB", "dml", insert, "args", slice).
									Fatalf("%+v", errors.Wrap(err, "can't perform DML"))
							}
						}
					}

					if err := tx.Commit(); err != nil {
						log.With("backend", "Icinga DB").Fatalf("%+v", errors.Wrap(err, "can't commit transaction"))
					}

					inc.inc(args[0].Len())
					return res[2:]
				},
			).Interface(),
		)
	})

	progress.Wait()
}
