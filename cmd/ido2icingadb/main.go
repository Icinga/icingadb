package main

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"fmt"
	"github.com/goccy/go-yaml"
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
	// Icinga2 specifies information the IDO doesn't provide.
	Icinga2 struct {
		// Env specifies the "Environment" config constant value (likely "").
		Env string `yaml:"env"`
		// Endpoint specifies the name on the main endpoint writing to IDO.
		Endpoint string `yaml:"endpoint"`
	} `yaml:"icinga2"`
}

// main validates the CLI, parses the config and migrates history from IDO to Icinga DB (see comments below).
// Most of the called functions exit the whole program by themselves on non-recoverable errors.
func main() {
	f := &Flags{}
	if _, err := flags.NewParser(f, flags.Default).Parse(); err != nil {
		os.Exit(2)
	}

	c, ex := parseConfig(f)
	if c == nil {
		os.Exit(ex)
	}

	defer func() { _ = log.Sync() }()

	log.Info("Starting IDO to Icinga DB history migration")

	ido, idb := connectAll(c)

	// Start repeatable-read-isolated transactions (consistent SELECTs)
	// not to have to care for IDO data changes during migration.
	startIdoTx(ido)

	// Prepare the directory structure the following fillCache() will need later.
	mkCache(f, idb.Mapper)

	log.Info("Computing progress")

	// Count total source data, so the following computeProgress()
	// knows how many work is left and can display progress bars.
	countIdoHistory()

	// Roll out a red carpet for the following computeProgress()' progress bars.
	_ = log.Sync()

	// computeProgress figures out which data has already been migrated
	// not to start from the beginning every time in the following migrate().
	computeProgress(c, idb)

	// On rationale read buildEventTimeCache() and buildPreviousHardStateCache() docs.
	log.Info("Filling cache")
	fillCache()

	log.Info("Actually migrating")
	migrate(c, idb)
}

// parseConfig validates the f.Config file and returns the config and -1 or - on failure - nil and an exit code.
func parseConfig(f *Flags) (_ *Config, exit int) {
	cf, err := os.Open(f.Config)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "can't open config file: %s\n", err.Error())
		return nil, 2
	}
	defer func() { _ = cf.Close() }()

	c := &Config{}
	if err := yaml.NewDecoder(cf).Decode(c); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "can't parse config file: %s\n", err.Error())
		return nil, 2
	}

	return c, -1
}

// mkCache ensures <f.Cache>/<history type>.sqlite3 files are present and contain their schema
// and initializes types[*].cache. (On non-recoverable errors the whole program exits.)
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

// connectAll connects to ido and idb (Icinga DB) as c specifies. (On non-recoverable errors the whole program exits.)
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

// connect connects to which DB as cfg specifies. (On non-recoverable errors the whole program exits.)
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

// startIdoTx initializes types[*].snapshot with new repeatable-read-isolated ido transactions.
// (On non-recoverable errors the whole program exits.)
func startIdoTx(ido *icingadb.DB) {
	types.forEach(func(ht *historyType) {
		tx, err := ido.BeginTxx(context.Background(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
		if err != nil {
			log.Fatalf("%+v", errors.Wrap(err, "can't begin snapshot transaction"))
		}

		ht.snapshot = tx
	})
}

// countIdoHistory initializes types[*].total with how many events to migrate.
// (On non-recoverable errors the whole program exits.)
func countIdoHistory() {
	types.forEach(func(ht *historyType) {
		err := ht.snapshot.Get(
			&ht.total,
			// For actual migration icinga_objects will be joined anyway,
			// so it makes no sense to take vanished objects into account.
			"SELECT COUNT(*) FROM "+ht.idoTable+" xh INNER JOIN icinga_objects o ON o.object_id=xh.object_id",
		)
		if err != nil {
			log.Fatalf("%+v", errors.Wrap(err, "can't count query"))
		}

		log.Infow("Counted total IDO events", "type", ht.name, "amount", ht.total)
	})
}

// computeProgress initializes types[*].lastId with how many events have already been migrated to idb.
// (On non-recoverable errors the whole program exits.)
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
			// For actual migration icinga_objects will be joined anyway,
			// so it makes no sense to take vanished objects into account.
			ht.idoTable + " xh USE INDEX (PRIMARY) INNER JOIN icinga_objects o ON o.object_id=xh.object_id WHERE " +
			ht.idoIdColumn + " > :checkpoint ORDER BY xh." + // requires migrate() to migrate serially, in order
			ht.idoIdColumn + " LIMIT :bulk"

		// As long as the current chunk is lastRowsLen long (doesn't change)...
		var lastRowsLen int

		// ... we can re-use these:
		var lastQuery string
		var lastStmt *sqlx.Stmt

		baseIdbQuery := fmt.Sprintf("SELECT %s FROM %s WHERE %s IN (?)", ht.idbIdColumn, ht.idbTable, ht.idbIdColumn)
		inc := barIncrementer{ht.bar, time.Now()}

		defer func() {
			if lastStmt != nil {
				_ = lastStmt.Close()
			}
		}()

		// Stream IDO IDs, ...
		sliceIdoHistory(ht.snapshot, query, nil, 0, func(rows []ProgressRow) (checkpoint interface{}) {
			type convertedId struct {
				ido uint64
				idb []byte
			}

			ids := make([]interface{}, 0, len(rows))
			converted := make([]convertedId, 0, len(rows))

			// ... convert them to Icinga DB ones, ...
			for _, row := range rows {
				conv := ht.convertId(row, c.Icinga2.Env)
				ids = append(ids, conv)
				converted = append(converted, convertedId{row.Id, conv})
			}

			// ... prepare a new query if the last one doesn't fit, ...
			if len(rows) != lastRowsLen {
				if lastStmt != nil {
					_ = lastStmt.Close()
				}

				lastRowsLen = len(rows)

				{
					var err error
					lastQuery, _, err = sqlx.In(baseIdbQuery, ids...)

					if err != nil {
						log.With("query", baseIdbQuery).Fatalf("%+v", errors.Wrap(err, "can't assemble query"))
					}
				}

				var err error
				lastStmt, err = idb.Preparex(lastQuery)

				if err != nil {
					log.With("query", lastQuery).Fatalf("%+v", errors.Wrap(err, "can't prepare query"))
				}
			}

			// ... select which have already been migrated, ...
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

			// ... and in IDO ID order:
			for _, conv := range converted {
				var key [20]byte
				copy(key[:], conv.idb)

				// Stop on the first not yet migrated.
				if _, ok := presentSet[key]; !ok {
					return nil
				}

				// If an ID has already been migrated, increase the actual migration's start point.
				ht.lastId = conv.ido
			}

			inc.inc(len(rows))
			return ht.lastId
		})

		ht.bar.SetTotal(ht.bar.Current(), true)
	})

	progress.Wait()
}

// fillCache fills <f.Cache>/<history type>.sqlite3 (actually types[*].cacheFiller does).
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

// tAny can't be created by just using reflect.TypeOf(interface{}(nil)).
var tAny = reflect.TypeOf((*interface{})(nil)). // *interface{}
						Elem() // interface{}

// migrate does the actual migration.
func migrate(c *Config, idb *icingadb.DB) {
	envId := sha1.Sum([]byte(c.Icinga2.Env))
	endpointId := sha1.Sum([]byte(c.Icinga2.Endpoint))

	progress := mpb.New()
	for i := range types {
		types[i].setupBar(progress)
	}

	types.forEach(func(ht *historyType) {
		// type rowStructPtr = interface{}
		// type table = []rowStructPtr
		// func convertRowsFromIdoToIcingaDb(env string, envId, endpointId Binary,
		// selectCache func(dest interface{}, query string, args ...interface{}), ido *sqlx.Tx, idoRows []T)
		// (icingaDbUpdates, icingaDbInserts []table, checkpoint interface{})
		vConvertRows := reflect.ValueOf(ht.convertRows) // TODO: make historyType#convertRows generic[T] one nice day

		// []T (idoRows)
		tRows := vConvertRows.Type().In(5)

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
			reflect.ValueOf(func(dest interface{}, query string, args ...interface{}) { // selectCache
				// Prepare new one, if old one doesn't fit anymore.
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
			// and the rows (below)
		}

		var args map[string]interface{}

		// For the case that the cache was older that the IDO,
		// but ht.cacheFiller couldn't update it, limit (WHERE) our source data set.
		if ht.cacheLimitQuery != "" {
			var limit uint64
			cacheGet(ht.cache, &limit, ht.cacheLimitQuery)
			args = map[string]interface{}{"cache_limit": limit}
		}

		icingaDbInserts := map[reflect.Type]string{}
		icingaDbUpdates := map[reflect.Type]string{}
		inc := barIncrementer{ht.bar, time.Now()}

		// Stream IDO rows, ...
		sliceIdoHistory(
			ht.snapshot, ht.migrationQuery, args, ht.lastId,
			reflect.MakeFunc(reflect.FuncOf([]reflect.Type{tRows}, []reflect.Type{tAny}, false),
				func(args []reflect.Value) []reflect.Value {
					vConvertRowsArgs[5] = args[0] // pass the rows

					// ... convert them, ...
					res := vConvertRows.Call(vConvertRowsArgs[:])

					// ... and insert them:

					tx, err := idb.Beginx()
					if err != nil {
						log.With("backend", "Icinga DB").Fatalf("%+v", errors.Wrap(err, "can't begin transaction"))
					}

					for _, operation := range [...]struct {
						data      reflect.Value
						buildStmt func(subject interface{}) (stmt string, _ int)
						stmtCache map[reflect.Type]string
					}{{res[1], idb.BuildUpsertStmt, icingaDbInserts}, {res[0], idb.BuildUpdateStmt, icingaDbUpdates}} {
						for _, table := range operation.data.Interface().([][]interface{}) {
							if len(table) < 1 {
								continue
							}

							tRow := reflect.TypeOf(table[0])

							query, ok := operation.stmtCache[tRow]
							if !ok {
								query, _ = operation.buildStmt(table[0])
								operation.stmtCache[tRow] = query
							}

							stmt, err := tx.PrepareNamed(query)
							if err != nil {
								log.With("backend", "Icinga DB", "dml", query).
									Fatalf("%+v", errors.Wrap(err, "can't prepare DML"))
							}

							for _, row := range table {
								if _, err := stmt.Exec(row); err != nil {
									log.With("backend", "Icinga DB", "dml", query, "args", row).
										Fatalf("%+v", errors.Wrap(err, "can't perform DML"))
								}
							}

							_ = stmt.Close()
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
