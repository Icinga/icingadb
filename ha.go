package icingadb_ha_lib

import (
	"database/sql"
	"git.icinga.com/icingadb-connection"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"sync/atomic"
	"time"
)

const (
	// readyForTakeover says that we aren't responsible, but we could take over.
	resp_ReadyForTakeover = iota
	// TakeoverNoSync says that we've taken over, but we aren't actually syncing config, yet.
	resp_TakeoverNoSync
	// TakeoverSync says that we've taken over and are actually syncing config.
	resp_TakeoverSync
	// stop says that we've taken over and are actually syncing config, but we're going to stop it.
	resp_Stop
	// notReadyForTakeover says that we aren't responsible and can't take over.
	resp_NotReadyForTakeover
)

const (
	// noAction says that we won't do anything.
	action_NoAction = iota
	// tryTakeover says that we're going to try to take over.
	action_TryTakeover
	// doTakeover says that we're going to take over.
	action_DoTakeover
	// ceaseOperation says that we're going to release our responsibility.
	action_CeaseOperation
)

type HA struct {
	ourUUID      uuid.UUID
	icinga2MTime int64
	// responsibility tells whether we're responsible for our environment.
	responsibility uint32
	// responsibleSince tells since when we're responsible for our environment.
	responsibleSince time.Time
	// runningCriticalOperations counts the currently running critical operations.
	runningCriticalOperations uint64
	// lastCriticalOperationEnd tells when the last critical operation finished.
	lastCriticalOperationEnd int64
}

// RunCriticalOperation runs op and manages HA#runningCriticalOperations if we're responsible.
func (h *HA) RunCriticalOperation(op func() error) error {
	switch h.getResponsibility() {
	case resp_TakeoverSync, resp_Stop:
		atomic.AddUint64(&h.runningCriticalOperations, 1)

		err := op()

		atomic.StoreInt64(&h.lastCriticalOperationEnd, time.Now().Unix())
		atomic.AddUint64(&h.runningCriticalOperations, ^uint64(0))

		return err
	}

	return nil
}

func (h *HA) Icinga2HeartBeat() {
	atomic.StoreInt64(&h.icinga2MTime, time.Now().Unix())
}

func (h *HA) IsResponsible() bool {
	return h.getResponsibility() == resp_TakeoverSync
}

func (h *HA) Run(rdb *icingadb_connection.RDBWrapper, dbw *icingadb_connection.DBWrapper, chEnv chan *icingadb_connection.Environment, chErr chan error) {
	go cleanUpInstancesAsync(dbw, chErr)

	if errRun := h.run(rdb, dbw, chEnv); errRun != nil {
		chErr <- errRun
		return
	}
}

// cleanUpInstancesAsync cleans up icingadb_instance periodically.
func cleanUpInstancesAsync(dbw *icingadb_connection.DBWrapper, chErr chan error) {
	every5m := time.NewTicker(5 * time.Minute)
	defer every5m.Stop()

	for {
		<-every5m.C

		if errCI := cleanUpInstances(dbw); errCI != nil {
			chErr <- errCI
		}
	}
}

// cleanUpInstances cleans up icingadb_instance periodically.
func cleanUpInstances(dbw *icingadb_connection.DBWrapper) error {

	log.WithFields(log.Fields{"context": "HA"}).Info("Cleaning up icingadb_instance")

	errTx := dbw.SqlTransaction(true, true, func(tx *sql.Tx) error {
		_, errExec := dbw.SqlExec(
			tx,
			"delete from icingadb_instance by heartbeat",
			`DELETE FROM icingadb_instance WHERE ? - heartbeat >= 30`,
			time.Now().Unix(),
		)

		return errExec
	})
	return errTx
}

func (h *HA) run(rdb *icingadb_connection.RDBWrapper, dbw *icingadb_connection.DBWrapper, chEnv chan *icingadb_connection.Environment) error {
	log.WithFields(log.Fields{"context": "HA"}).Info("Waiting for Icinga 2 to tell us its environment")

	var env *icingadb_connection.Environment = nil
	var hasEnv bool

	env, hasEnv = <-chEnv
	if !hasEnv {
		return nil
	}

	var err error
	if h.ourUUID, err = uuid.NewRandom(); err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"context": "HA",
		"uuid":    h.ourUUID.String(),
		"env":     env.Name,
	}).Info("Received environment from Icinga 2")

	everySecond := time.NewTicker(time.Second)
	defer everySecond.Stop()

	var nextAction = 0
	var theirUUID uuid.UUID

	// Even if Icinga 2 is offline now, Redis may be filled
	h.Icinga2HeartBeat()

	for {
		switch h.getResponsibility() {
		case resp_ReadyForTakeover:
			if !h.icinga2IsAlive() {
				log.WithFields(log.Fields{
					"context": "HA",
					"uuid":    h.ourUUID.String(),
					"env":     env.Name,
				}).Warn("Icinga 2 detected as not running, stopping.")

				h.setResponsibility(resp_NotReadyForTakeover)
				continue
			}

			nextAction = action_TryTakeover
		case resp_TakeoverNoSync:
			if !h.icinga2IsAlive() {
				log.WithFields(log.Fields{
					"context": "HA",
					"uuid":    h.ourUUID.String(),
					"env":     env.Name,
				}).Warn("Icinga 2 detected as not running, stopping.")

				h.setResponsibility(resp_Stop)
				continue
			}

			nextAction = action_TryTakeover
		case resp_TakeoverSync:
			if !h.icinga2IsAlive() {
				log.WithFields(log.Fields{
					"context": "HA",
					"uuid":    h.ourUUID.String(),
					"env":     env.Name,
				}).Warn("Icinga 2 detected as not running, stopping.")

				h.setResponsibility(resp_Stop)
				continue
			}

			nextAction = action_DoTakeover
		case resp_Stop:
			if atomic.LoadUint64(&h.runningCriticalOperations) == 0 && time.Now().Unix()-atomic.LoadInt64(&h.lastCriticalOperationEnd) >= 5 {
				nextAction = action_CeaseOperation
			} else {
				nextAction = action_DoTakeover
			}
		case resp_NotReadyForTakeover:
			if h.icinga2IsAlive() {
				log.WithFields(log.Fields{
					"context": "HA",
					"uuid":    h.ourUUID.String(),
					"env":     env.Name,
				}).Info("Icinga 2 detected as running again.")

				h.setResponsibility(resp_ReadyForTakeover)
				continue
			}

			nextAction = action_NoAction
		}

		switch nextAction {
		case action_NoAction:
			break
		case action_TryTakeover, action_DoTakeover:
			var justTakenOver bool

			errTx := dbw.SqlTransactionQuiet(true, true, func(tx *sql.Tx) error {
				{
					rows, errFA := dbw.SqlFetchAllQuiet(
						tx,
						"select from icingadb_instance by id",
						`SELECT 1 FROM icingadb_instance WHERE id = ?`,
						h.ourUUID[:],
					)
					if errFA != nil {
						return errFA
					}

					if len(rows) > 0 {
						_, errExec := dbw.SqlExecQuiet(
							tx,
							"update icingadb_instance by id",
							`UPDATE icingadb_instance SET environment_id=?, heartbeat=? WHERE id = ?`,
							env.ID,
							time.Now().Unix(),
							h.ourUUID[:],
						)
						if errExec != nil {
							return errExec
						}
					} else {
						_, errExec := dbw.SqlExecQuiet(
							tx,
							"insert into icingadb_instance",
							`INSERT INTO icingadb_instance(id, environment_id, heartbeat, responsible) VALUES (?, ?, ?, ?)`,
							h.ourUUID[:],
							env.ID,
							time.Now().Unix(),
							"n",
						)
						if errExec != nil {
							return errExec
						}
					}
				}

				justTakenOver = false

				rows, errFA := dbw.SqlFetchAllQuiet(
					tx,
					"select from icingadb_instance by environment_id, responsible",
					`SELECT id, heartbeat FROM icingadb_instance WHERE environment_id = ? AND responsible = ?`,
					env.ID,
					"y",
				)
				if errFA != nil {
					return errFA
				}

				if len(rows) > 0 {
					copy(theirUUID[:], rows[0][0].([]byte))

					if theirUUID == h.ourUUID {
						justTakenOver = true
					} else if time.Now().Unix()-rows[0][1].(int64) >= 10 {
						{
							_, errExec := dbw.SqlExecQuiet(
								tx,
								"update icingadb_instance by environment_id",
								`UPDATE icingadb_instance SET responsible=? WHERE environment_id = ?`,
								"n",
								env.ID,
							)
							if errExec != nil {
								return errExec
							}
						}

						_, errExec := dbw.SqlExecQuiet(
							tx,
							"update icingadb_instance by id",
							`UPDATE icingadb_instance SET responsible=? WHERE id = ?`,
							"y",
							h.ourUUID[:],
						)
						if errExec != nil {
							return errExec
						}

						justTakenOver = true
					}
				} else {
					_, errExec := dbw.SqlExecQuiet(
						tx,
						"update icingadb_instance by id",
						`UPDATE icingadb_instance SET responsible=? WHERE id = ?`,
						"y",
						h.ourUUID[:],
					)
					if errExec != nil {
						return errExec
					}

					justTakenOver = true
				}

				return nil
			})
			if errTx != nil {
				return errTx
			}

			if justTakenOver && h.getResponsibility() != resp_Stop {
				if h.responsibleSince == (time.Time{}) {
					h.responsibleSince = time.Now()
					h.setResponsibility(resp_TakeoverNoSync)
				} else {
					responsibleFor := time.Now().Sub(h.responsibleSince).Seconds()

					if responsibleFor >= 5.0 {
						if h.setResponsibility(resp_TakeoverSync) == resp_TakeoverNoSync {
							log.WithFields(log.Fields{
								"context":    "HA",
								"env":        env.Name,
								"their_uuid": theirUUID.String(),
							}).Info("Taking over")
						}

						if _, errRP := rdb.Publish("icingadb:wakeup", h.ourUUID.String()).Result(); errRP != nil {
							return errRP
						}
					}
				}
			}

			if !justTakenOver {
				log.WithFields(log.Fields{
					"context":    "HA",
					"env":        env.Name,
					"their_uuid": theirUUID.String(),
				}).Info("Other instance is responsible")
			}
		case action_CeaseOperation:
			errTx := dbw.SqlTransactionQuiet(true, true, func(tx *sql.Tx) error {
				rows, errFA := dbw.SqlFetchAllQuiet(
					tx,
					"select from icingadb_instance by environment_id, responsible, heartbeat",
					`SELECT 1 FROM icingadb_instance WHERE environment_id = ? AND responsible = ? AND ? - heartbeat < 10`,
					env.ID,
					"n",
					time.Now().Unix(),
				)
				if errFA != nil {
					return errFA
				}

				if len(rows) > 0 {
					_, errExec := dbw.SqlExecQuiet(
						tx,
						"update icingadb_instance",
						`UPDATE icingadb_instance SET responsible=? WHERE id = ?`,
						"n",
						h.ourUUID[:],
					)

					return errExec
				}

				return nil
			})
			if errTx != nil {
				return errTx
			}

			log.WithFields(log.Fields{
				"context": "HA",
				"env":     env.Name,
			}).Info("Other instance is responsible. Ceasing operations.")

			h.responsibleSince = time.Time{}
			h.setResponsibility(resp_NotReadyForTakeover)
		}

		select {
		case env, hasEnv = <-chEnv:
			if !hasEnv {
				return nil
			}

			<-everySecond.C
		case <-everySecond.C:
			break
		}
	}
}

// icinga2IsAlive returns whether Icinga 2 seems to be running.
func (h *HA) icinga2IsAlive() bool {
	return time.Now().Unix()-atomic.LoadInt64(&h.icinga2MTime) < 15
}

// getResponsibility gets the responsibility.
func (h *HA) getResponsibility() uint32 {
	return atomic.LoadUint32(&h.responsibility)
}

// setResponsibility sets the responsibility and returns the previous one.
func (h *HA) setResponsibility(r uint32) uint32 {
	return atomic.SwapUint32(&h.responsibility, uint32(r))
}
