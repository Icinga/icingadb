package cleanup

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/utils"
	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"gopkg.in/ini.v1"
	"sync"
	"time"
)

type Cleanup struct {
	wg           *sync.WaitGroup
	dbw          *connection.DBWrapper
	tick         time.Duration
	limit        int
}


type Tableconfig struct {
	Table     string
	Period    time.Duration
	Starttime time.Time
}

type Cleanupfunc func(ctx context.Context, tbl Tableconfig, dbw *connection.DBWrapper, limit int) (sql.Result, error)

type Cleanfuncmap map[string]Cleanupfunc

func NewCleanup(dbw *connection.DBWrapper) *Cleanup {
	return &Cleanup{
		dbw: dbw,
		wg: &sync.WaitGroup{},
		tick: time.Hour,
		limit: 5000,
	}
}

func (c *Cleanup) Start() error {

	configPath := flag.String("cleanup", "cleanup/cleanup.ini", "path to config")
	flag.Parse()

	cfg, err := ini.Load(*configPath)
	if err != nil {
		return err
	}

	tables := cfg.Section("cleanup").KeysHash()

	var cufuncConfig = Cleanfuncmap{
		"acknowledgement_history": cleanAckFunc,
		"comment_history": cleanCommentFunc,
		"downtime_history": cleanDowntimeFunc,
		"flapping_history": cleanFlappingFunc,
		"notification_history": cleanNotificationFunc,
		"state_history": cleanStateFunc,
	}

	ctx := context.Background()
	for table, period := range tables {
		tempPeriod, _ := time.ParseDuration(period)
		c.wg.Add(1)
		go c.controller(ctx, Tableconfig{table, tempPeriod, time.Now()}, cufuncConfig[table])
	}

	c.wg.Wait()
	return nil
}

func (c *Cleanup) controller(ctx context.Context, tbl Tableconfig, cufunc Cleanupfunc) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		err, _ := c.cleanup(ctx, tbl, cufunc)
		if err != nil {
			cancel()
		}
		select {
		case <-time.Tick(c.tick):
		case <-ctx.Done():
			log.Error(err)
			c.wg.Done()
			return
		}
	}
}

func (c *Cleanup) cleanup(ctx context.Context, tbl Tableconfig, cufunc Cleanupfunc) (error, int) {
	redo := true

	rowsAffected := make(chan int64, 1)
	defer close(rowsAffected)

	errs := make(chan error, 1)
	defer close(errs)
	limit := c.limit

	tName := tbl.Table
	numdel := 0
	for redo {
		go func() {
			result, err := cufunc(ctx, tbl, c.dbw, limit)

			if err != nil {
				errs <- err
				return
			}
			affected, err := result.RowsAffected()

			if err != nil {
				errs <- err
				return
			}
			log.Infof("Rows affected in %s: %v", tName, affected)
			rowsAffected <- affected
		}()

		select {
		case affected:= <-rowsAffected:
			numdel++
			if affected < int64(limit) {
				redo = false
			}
			limit = c.limit
		case <-time.After(time.Second):
			limit = limit/2
		case err := <-errs:
			return err, numdel
		}
	}
	return nil, numdel
}

func cleanAckFunc(ctx context.Context, tblcfg Tableconfig, dbw *connection.DBWrapper, limit int) (sql.Result, error){
	eventtime := utils.TimeToMillisecs(tblcfg.Starttime.Add(-1*tblcfg.Period))

	result, err := dbw.Db.ExecContext(
		ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE set_time < ? AND (clear_time IS NOT NULL AND clear_time < ?) LIMIT %d`, tblcfg.Table, limit),
		eventtime, eventtime)

	return result, err
}

func cleanCommentFunc(ctx context.Context, tblcfg Tableconfig, dbw *connection.DBWrapper, limit int) (sql.Result, error){
	eventtime := utils.TimeToMillisecs(tblcfg.Starttime.Add(-1*tblcfg.Period))
	result, err := dbw.Db.ExecContext(
		ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE entry_time < ? AND (remove_time IS NOT NULL AND remove_time < ?) LIMIT %d`, tblcfg.Table, limit),
		eventtime, eventtime)

	return result, err
}

func cleanDowntimeFunc(ctx context.Context, tblcfg Tableconfig, dbw *connection.DBWrapper, limit int) (sql.Result, error){
	eventtime := utils.TimeToMillisecs(tblcfg.Starttime.Add(-1*tblcfg.Period))

	result, err := dbw.Db.ExecContext(
		ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE start_time < ? AND (end_time IS NOT NULL AND end_time < ?) LIMIT %d`, tblcfg.Table, limit),
		eventtime, eventtime)

	return result, err
}

func cleanFlappingFunc(ctx context.Context, tblcfg Tableconfig, dbw *connection.DBWrapper, limit int) (sql.Result, error){
	eventtime := utils.TimeToMillisecs(tblcfg.Starttime.Add(-1*tblcfg.Period))

	result, err := dbw.Db.ExecContext(
		ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE start_time < ? AND (end_time IS NOT NULL AND end_time < ?) LIMIT %d`, tblcfg.Table, limit),
		eventtime, eventtime)

	return result, err
}


func cleanNotificationFunc(ctx context.Context, tblcfg Tableconfig, dbw *connection.DBWrapper, limit int) (sql.Result, error){
	result, err := dbw.Db.ExecContext(
		ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE send_time < ? LIMIT %d`, tblcfg.Table, limit),
		utils.TimeToMillisecs(tblcfg.Starttime.Add(-1*tblcfg.Period)))

	return result, err
}

func cleanStateFunc(ctx context.Context, tblcfg Tableconfig, dbw *connection.DBWrapper, limit int) (sql.Result, error){
	result, err := dbw.Db.ExecContext(
		ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE event_time < ? LIMIT %d`, tblcfg.Table, limit),
		utils.TimeToMillisecs(tblcfg.Starttime.Add(-1*tblcfg.Period)))

	return result, err
}
