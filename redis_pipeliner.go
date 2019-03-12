package icingadb_connection

import "github.com/go-redis/redis"

type PipelinerWrapper struct {
	pipeliner	redis.Pipeliner
	rdbw 		*RDBWrapper
}

func (plw *PipelinerWrapper) Exec() ([]redis.Cmder, error) {
	for {
		if !plw.rdbw.IsConnected() {
			plw.rdbw.WaitForConnection()
			continue
		}

		cmder, err := plw.pipeliner.Exec()

		if err != nil {
			if !plw.rdbw.CheckConnection(false) {
				continue
			}
		}

		return cmder, err
	}
}

func (plw *PipelinerWrapper) HMGet(key string, fields ...string) *redis.SliceCmd {
	for {
		if !plw.rdbw.IsConnected() {
			plw.rdbw.WaitForConnection()
			continue
		}

		cmd := plw.pipeliner.HMGet(key, fields...)
		_, err := cmd.Result()

		if err != nil {
			if !plw.rdbw.CheckConnection(false) {
				continue
			}
		}

		return cmd
	}
}