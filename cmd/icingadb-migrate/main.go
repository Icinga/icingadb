package main

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"fmt"
	"github.com/creasty/defaults"
	"github.com/goccy/go-yaml"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-go-library/utils"
	"github.com/icinga/icingadb/pkg/icingadb"
	"github.com/jessevdk/go-flags"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/reflectx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/vbauerster/mpb/v6"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
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
	IDO struct {
		database.Config `yaml:"-,inline"`
		From            int32 `yaml:"from"`
		To              int32 `yaml:"to" default:"2147483647"`
	} `yaml:"ido"`
	IcingaDB database.Config `yaml:"icingadb"`
	// Icinga2 specifies information the IDO doesn't provide.
	Icinga2 struct {
		// Env specifies the environment ID, hex.
		Env string `yaml:"env"`
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
	if len(envId) != 20 {
		_, _ = fmt.Fprintf(os.Stderr, "bad env ID: must be 20 bytes long, has %d bytes\n", len(envId))
		os.Exit(2)
	}

	defer func() { _ = log.Sync() }()

	log.Info("Starting IDO to Icinga DB history migration")

	ido, idb := connectAll(c)

	if err := icingadb.CheckSchema(context.Background(), idb); err != nil {
		log.Fatalf("%+v", err)
	}

	// Start repeatable-read-isolated transactions (consistent SELECTs)
	// not to have to care for IDO data changes during migration.
	startIdoTx(ido)

	// Prepare the directory structure the following fillCache() will need later.
	mkCache(f, c, idb.Mapper)

	log.Info("Computing progress")

	// Convert Config#IDO.From and .To to IDs to restrict data by PK.
	computeIdRange(c)

	// computeProgress figures out which data has already been migrated
	// not to start from the beginning every time in the following migrate().
	computeProgress(c, idb, envId)

	// On rationale read buildEventTimeCache() and buildPreviousHardStateCache() docs.
	log.Info("Filling cache")
	fillCache()

	log.Info("Actually migrating")
	migrate(c, idb, envId)

	log.Info("Cleaning up cache")
	cleanupCache(f)
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
	if err := defaults.Set(c); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "can't set config defaults: %s\n", err.Error())
		return nil, 2
	}

	if err := yaml.NewDecoder(cf, yaml.DisallowUnknownField()).Decode(c); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "can't parse config file: %s\n", err.Error())
		return nil, 2
	}

	return c, -1
}

var nonWords = regexp.MustCompile(`\W+`)

// mkCache ensures <f.Cache>/<history type>.sqlite3 files are present and contain their schema
// and initializes typesToMigrate[*].cache. (On non-recoverable errors the whole program exits.)
func mkCache(f *Flags, c *Config, mapper *reflectx.Mapper) {
	log.Info("Preparing cache")

	if err := os.MkdirAll(f.Cache, 0700); err != nil {
		log.With("dir", f.Cache).Fatalf("%+v", errors.Wrap(err, "can't create directory"))
	}

	typesToMigrate.forEach(func(ht *historyType) {
		if ht.cacheSchema == "" {
			return
		}

		file := path.Join(f.Cache, fmt.Sprintf(
			"%s_%d-%d.sqlite3", nonWords.ReplaceAllLiteralString(ht.name, "_"), c.IDO.From, c.IDO.To,
		))

		var err error

		ht.cache, err = sqlx.Open("sqlite3", "file:"+file)
		if err != nil {
			log.With("file", file).Fatalf("%+v", errors.Wrap(err, "can't open SQLite database"))
		}

		ht.cacheFile = file
		ht.cache.Mapper = mapper

		if _, err := ht.cache.Exec(ht.cacheSchema); err != nil {
			log.With("file", file, "ddl", ht.cacheSchema).
				Fatalf("%+v", errors.Wrap(err, "can't import schema into SQLite database"))
		}
	})
}

// connectAll connects to ido and idb (Icinga DB) as c specifies. (On non-recoverable errors the whole program exits.)
func connectAll(c *Config) (ido, idb *database.DB) {
	log.Info("Connecting to databases")
	eg, _ := errgroup.WithContext(context.Background())

	eg.Go(func() error {
		ido = connect("IDO", &c.IDO.Config)
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
func connect(which string, cfg *database.Config) *database.DB {
	db, err := database.NewDbFromConfig(
		cfg,
		logging.NewLogger(zap.NewNop().Sugar(), 20*time.Second),
		database.RetryConnectorCallbacks{},
	)
	if err != nil {
		log.With("backend", which).Fatalf("%+v", errors.Wrap(err, "can't connect to database"))
	}

	if err := db.Ping(); err != nil {
		log.With("backend", which).Fatalf("%+v", errors.Wrap(err, "can't connect to database"))
	}

	return db
}

// startIdoTx initializes typesToMigrate[*].snapshot with new repeatable-read-isolated ido transactions.
// (On non-recoverable errors the whole program exits.)
func startIdoTx(ido *database.DB) {
	typesToMigrate.forEach(func(ht *historyType) {
		tx, err := ido.BeginTxx(context.Background(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
		if err != nil {
			log.Fatalf("%+v", errors.Wrap(err, "can't begin snapshot transaction"))
		}

		ht.snapshot = tx
	})
}

// computeIdRange initializes typesToMigrate[*].fromId and typesToMigrate[*].toId.
// (On non-recoverable errors the whole program exits.)
func computeIdRange(c *Config) {
	typesToMigrate.forEach(func(ht *historyType) {
		getBorderId := func(id *uint64, timeColumns []string, compOperator string, borderTime int32, sortOrder string) {
			deZeroFied := make([]string, 0, len(timeColumns))
			for _, column := range timeColumns {
				deZeroFied = append(deZeroFied, fmt.Sprintf(
					"CASE WHEN %[1]s < '1970-01-03 00:00:00' THEN NULL ELSE %[1]s END", column,
				))
			}

			var timeExpr string
			if len(deZeroFied) > 1 {
				timeExpr = "COALESCE(" + strings.Join(deZeroFied, ",") + ")"
			} else {
				timeExpr = deZeroFied[0]
			}

			query := ht.snapshot.Rebind(
				"SELECT " + ht.idoIdColumn + " FROM " + ht.idoTable + " WHERE " + timeExpr + " " + compOperator +
					" FROM_UNIXTIME(?) ORDER BY " + ht.idoIdColumn + " " + sortOrder + " LIMIT 1",
			)

			switch err := ht.snapshot.Get(id, query, borderTime); err {
			case nil, sql.ErrNoRows:
			default:
				log.With("backend", "IDO", "query", query, "args", []any{borderTime}).
					Fatalf("%+v", errors.Wrap(err, "can't perform query"))
			}
		}

		ht.fromId = math.MaxInt64

		getBorderId(&ht.fromId, ht.idoEndColumns, ">=", c.IDO.From, "ASC")
		getBorderId(&ht.toId, ht.idoStartColumns, "<=", c.IDO.To, "DESC")
	})
}

//go:embed embed/ido_migration_progress_schema.sql
var idoMigrationProgressSchema string

// computeProgress initializes typesToMigrate[*].lastId, typesToMigrate[*].total and typesToMigrate[*].done.
// (On non-recoverable errors the whole program exits.)
func computeProgress(c *Config, idb *database.DB, envId []byte) {
	if _, err := idb.Exec(idoMigrationProgressSchema); err != nil {
		log.Fatalf("%+v", errors.Wrap(err, "can't create table ido_migration_progress"))
	}

	envIdHex := hex.EncodeToString(envId)
	typesToMigrate.forEach(func(ht *historyType) {
		var query = idb.Rebind(
			"SELECT last_ido_id FROM ido_migration_progress" +
				" WHERE environment_id=? AND history_type=? AND from_ts=? AND to_ts=?",
		)

		args := []any{envIdHex, ht.name, c.IDO.From, c.IDO.To}

		if err := idb.Get(&ht.lastId, query, args...); err != nil && err != sql.ErrNoRows {
			log.With("backend", "Icinga DB", "query", query, "args", args).
				Fatalf("%+v", errors.Wrap(err, "can't perform query"))
		}
	})

	typesToMigrate.forEach(func(ht *historyType) {
		if ht.cacheFiller != nil {
			err := ht.snapshot.Get(
				&ht.cacheTotal,
				ht.snapshot.Rebind(
					// For actual migration icinga_objects will be joined anyway,
					// so it makes no sense to take vanished objects into account.
					"SELECT COUNT(*) FROM "+ht.idoTable+
						" xh INNER JOIN icinga_objects o ON o.object_id=xh.object_id WHERE xh."+ht.idoIdColumn+" <= ?",
				),
				ht.toId,
			)
			if err != nil {
				log.Fatalf("%+v", errors.Wrap(err, "can't count query"))
			}
		}
	})

	typesToMigrate.forEach(func(ht *historyType) {
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
					ht.idoTable+" xh INNER JOIN icinga_objects o ON o.object_id=xh.object_id WHERE xh."+
					ht.idoIdColumn+" BETWEEN ? AND ? GROUP BY migrated",
			),
			ht.lastId, ht.fromId, ht.toId,
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

// fillCache fills <f.Cache>/<history type>.sqlite3 (actually typesToMigrate[*].cacheFiller does).
func fillCache() {
	progress := mpb.New()
	for _, ht := range typesToMigrate {
		if ht.cacheFiller != nil {
			ht.setupBar(progress, ht.cacheTotal)
		}
	}

	typesToMigrate.forEach(func(ht *historyType) {
		if ht.cacheFiller != nil {
			ht.cacheFiller(ht)
		}
	})

	progress.Wait()
}

// migrate does the actual migration.
func migrate(c *Config, idb *database.DB, envId []byte) {
	progress := mpb.New()
	for _, ht := range typesToMigrate {
		ht.setupBar(progress, ht.total)
	}

	typesToMigrate.forEach(func(ht *historyType) {
		ht.migrate(c, idb, envId, ht)
	})

	progress.Wait()
}

// migrate does the actual migration for one history type.
func migrateOneType[IdoRow any](
	c *Config, idb *database.DB, envId []byte, ht *historyType,
	convertRows func(env string, envId types.Binary,
		selectCache func(dest interface{}, query string, args ...interface{}), ido *sqlx.Tx,
		idoRows []IdoRow) (stages []icingaDbOutputStage, checkpoint any),
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
		var limit sql.NullInt64
		cacheGet(ht.cache, &limit, ht.cacheLimitQuery)
		args = map[string]interface{}{"cache_limit": limit.Int64}
	}

	upsertProgress, _ := idb.BuildUpsertStmt(&IdoMigrationProgress{})
	envIdHex := hex.EncodeToString(envId)

	ht.bar.SetCurrent(ht.done)

	// Stream IDO rows, ...
	sliceIdoHistory(
		ht, ht.migrationQuery, args, ht.lastId,
		func(idoRows []IdoRow) (checkpoint interface{}) {
			// ... convert them, ...
			stages, lastIdoId := convertRows(c.Icinga2.Env, envId, selectCache, ht.snapshot, idoRows)

			// ... and insert them:

			for _, stage := range stages {
				if len(stage.insert) > 0 {
					ch := utils.ChanFromSlice(stage.insert)

					if err := idb.CreateIgnoreStreamed(context.Background(), ch); err != nil {
						log.With("backend", "Icinga DB", "op", "INSERT IGNORE", "table", database.TableName(stage.insert[0])).
							Fatalf("%+v", errors.Wrap(err, "can't perform DML"))
					}
				}

				if len(stage.upsert) > 0 {
					ch := utils.ChanFromSlice(stage.upsert)

					if err := idb.UpsertStreamed(context.Background(), ch); err != nil {
						log.With("backend", "Icinga DB", "op", "UPSERT", "table", database.TableName(stage.upsert[0])).
							Fatalf("%+v", errors.Wrap(err, "can't perform DML"))
					}
				}
			}

			if lastIdoId != nil {
				args := map[string]interface{}{"history_type": ht.name, "last_ido_id": lastIdoId}

				_, err := idb.NamedExec(upsertProgress, &IdoMigrationProgress{
					IdoMigrationProgressUpserter{lastIdoId}, envIdHex, ht.name, c.IDO.From, c.IDO.To,
				})
				if err != nil {
					log.With("backend", "Icinga DB", "dml", upsertProgress, "args", args).
						Fatalf("%+v", errors.Wrap(err, "can't perform DML"))
				}
			}

			ht.bar.IncrBy(len(idoRows))
			return lastIdoId
		},
	)

	ht.bar.SetTotal(ht.bar.Current(), true)
}

// cleanupCache removes <f.Cache>/<history type>.sqlite3 files.
func cleanupCache(f *Flags) {
	typesToMigrate.forEach(func(ht *historyType) {
		if ht.cacheFile != "" {
			if err := ht.cache.Close(); err != nil {
				log.With("file", ht.cacheFile).Warnf("%+v", errors.Wrap(err, "can't close SQLite database"))
			}
		}
	})

	if matches, err := filepath.Glob(path.Join(f.Cache, "*.sqlite3")); err == nil {
		for _, match := range matches {
			if err := os.Remove(match); err != nil {
				log.With("file", match).Warnf("%+v", errors.Wrap(err, "can't remove SQLite database"))
			}
		}
	} else {
		log.With("dir", f.Cache).Warnf("%+v", errors.Wrap(err, "can't list SQLite databases"))
	}
}
