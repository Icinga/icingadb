package main

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"github.com/goccy/go-yaml"
	"github.com/icinga/icingadb/pkg/config"
	"github.com/icinga/icingadb/pkg/driver"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/icinga/icingadb/pkg/logging"
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
	"sync"
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
		// Env specifies the environment ID, hex.
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

	envId, err := hex.DecodeString(c.Icinga2.Env)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "bad env ID: %s\n", err.Error())
		os.Exit(2)
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

	// computeProgress figures out which data has already been migrated
	// not to start from the beginning every time in the following migrate().
	computeProgress(idb)

	// On rationale read buildEventTimeCache() and buildPreviousHardStateCache() docs.
	log.Info("Filling cache")
	fillCache()

	log.Info("Actually migrating")
	migrate(c, idb, envId)
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
	db, err := cfg.Open(logging.NewLogger(log, 20*time.Second))
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

// computeProgress initializes types[*].lastId, types[*].total and types[*].done.
// (On non-recoverable errors the whole program exits.)
func computeProgress(idb *icingadb.DB) {
	{
		_, err := idb.Exec(`CREATE TABLE IF NOT EXISTS ido_migration_progress (
    history_type VARCHAR(63) NOT NULL,
    last_ido_id BIGINT NOT NULL,

    CONSTRAINT pk_ido_migration_progress PRIMARY KEY (history_type)
)`)
		if err != nil {
			log.Fatalf("%+v", errors.Wrap(err, "can't create table ido_migration_progress"))
		}
	}

	stmt, _ := idb.BuildUpsertStmt(&IdoMigrationProgress{})

	types.forEach(func(ht *historyType) {
		row := &IdoMigrationProgress{IdoMigrationProgressUpserter{ht.name}, 0}

		if _, err := idb.NamedExec(stmt, row); err != nil {
			log.With("backend", "Icinga DB", "dml", stmt, "args", []interface{}{ht.name, 0}).
				Fatalf("%+v", errors.Wrap(err, "can't perform DML"))
		}

		var query = idb.Rebind("SELECT last_ido_id FROM ido_migration_progress WHERE history_type=?")

		if err := idb.Get(&ht.lastId, query, ht.name); err != nil {
			log.With("backend", "Icinga DB", "query", query, "args", []interface{}{ht.name}).
				Fatalf("%+v", errors.Wrap(err, "can't perform query"))
		}
	})

	types.forEach(func(ht *historyType) {
		var rows []struct {
			Migrated uint8
			Cnt      int64
		}

		err := ht.snapshot.Select(
			&rows,
			ht.snapshot.Rebind(
				// For actual migration icinga_objects will be joined anyway,
				// so it makes no sense to take vanished objects into account.
				"SELECT CASE WHEN xh."+ht.idoIdColumn+"<=? THEN 1 ELSE 0 END migrated, COUNT(*) cnt FROM "+
					ht.idoTable+" xh INNER JOIN icinga_objects o ON o.object_id=xh.object_id GROUP BY migrated",
			),
			ht.lastId,
		)
		if err != nil {
			log.Fatalf("%+v", errors.Wrap(err, "can't count query"))
		}

		for _, row := range rows {
			ht.total += row.Cnt

			if row.Migrated == 1 {
				ht.done = row.Cnt
			}
		}

		log.Infow("Counted migrated IDO events", "type", ht.name, "migrated", ht.done, "total", ht.total)
	})
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

// migrate does the actual migration.
func migrate(c *Config, idb *icingadb.DB, envId []byte) {
	endpointId := sha1.Sum([]byte(c.Icinga2.Endpoint))
	idbTx := &sync.Mutex{}

	progress := mpb.New()
	for i := range types {
		types[i].setupBar(progress)
	}

	types.forEach(func(ht *historyType) {
		ht.migrate(c, idb, envId, endpointId, idbTx, ht)
	})

	progress.Wait()
}

// migrate does the actual migration for one history type.
func migrateOneType[IdoRow any](
	c *Config, idb *icingadb.DB, envId []byte, endpointId [sha1.Size]byte, idbTx *sync.Mutex, ht *historyType,
	convertRows func(env string, envId, endpointId icingadbTypes.Binary,
		selectCache func(dest interface{}, query string, args ...interface{}), ido *sqlx.Tx,
		idoRows []IdoRow) (icingaDbUpdates, icingaDbInserts [][]interface{}, checkpoint interface{}),
) {
	var lastQuery string
	var lastStmt *sqlx.Stmt

	defer func() {
		if lastStmt != nil {
			_ = lastStmt.Close()
		}
	}()

	selectCache := func(dest interface{}, query string, args ...interface{}) {
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

	ht.bar.SetCurrent(ht.done)

	// Stream IDO rows, ...
	sliceIdoHistory(
		ht.snapshot, ht.migrationQuery, args, ht.lastId,
		func(idoRows []IdoRow) (checkpoint interface{}) {
			// ... convert them, ...
			updates, inserts, lastIdoId := convertRows(
				c.Icinga2.Env, envId, endpointId[:], selectCache, ht.snapshot, idoRows,
			)

			// ... and insert them:

			if idb.DriverName() == driver.MySQL {
				idbTx.Lock()
				defer idbTx.Unlock()
			}

			tx, err := idb.Beginx()
			if err != nil {
				log.With("backend", "Icinga DB").Fatalf("%+v", errors.Wrap(err, "can't begin transaction"))
			}

			for _, operation := range [...]struct {
				data      [][]interface{}
				buildStmt func(subject interface{}) (stmt string, _ int)
				stmtCache map[reflect.Type]string
			}{{inserts, idb.BuildUpsertStmt, icingaDbInserts}, {updates, idb.BuildUpdateStmt, icingaDbUpdates}} {
				for _, table := range operation.data {
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

			if lastIdoId != nil {
				const stmt = "UPDATE ido_migration_progress SET last_ido_id=:last_ido_id " +
					"WHERE history_type=:history_type"

				args := map[string]interface{}{"history_type": ht.name, "last_ido_id": lastIdoId}
				if _, err := tx.NamedExec(stmt, args); err != nil {
					log.With("backend", "Icinga DB", "dml", stmt, "args", args).
						Fatalf("%+v", errors.Wrap(err, "can't perform DML"))
				}
			}

			if err := tx.Commit(); err != nil {
				log.With("backend", "Icinga DB").Fatalf("%+v", errors.Wrap(err, "can't commit transaction"))
			}

			ht.bar.IncrBy(len(idoRows))
			return lastIdoId
		},
	)

	ht.bar.SetTotal(ht.bar.Current(), true)
}
