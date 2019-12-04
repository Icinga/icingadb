// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package ha

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"github.com/Icinga/icingadb/connection"
	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
)

type Environment struct {
	ID       []byte
	Name     string
	NodeName string
	Icinga2  Icinga2Info
}

type Icinga2Info struct {
	Version      string
	ProgramStart float64
	EndpointId   []byte
}

// Sha1bytes computes SHA1.
func Sha1bytes(bytes []byte) []byte {
	hash := sha1.New()
	hash.Write(bytes)
	return hash.Sum(nil)
}

func IcingaHeartbeatListener(rdb *connection.RDBWrapper, chEnv chan *Environment, chErr chan error) {
	log.Info("Starting heartbeat listener")

	xReadArgs := redis.XReadArgs{
		Streams: []string{"icinga:stats", "0-0"},
		Count:   1,
		Block:   0,
	}

	for {
		streams, errXR := rdb.XRead(&xReadArgs).Result()
		if errXR != nil {
			chErr <- errXR
			return
		}

		for _, stream := range streams {
			for _, message := range stream.Messages {
				log.Debug("Got heartbeat")

				xReadArgs.Streams[1] = message.ID

				if appJson, ok := message.Values["IcingaApplication"].(string); ok {
					var unJson struct {
						Status struct {
							IcingaApplication struct {
								App struct {
									Environment  string  `json:"environment"`
									NodeName     string  `json:"node_name"`
									Version      string  `json:"version"`
									ProgramStart float64 `json:"program_start"`
									EndpointId   string  `json:"endpoint_id"`
								} `json:"app"`
							} `json:"icingaapplication"`
						} `json:"status"`
					}

					if errJU := json.Unmarshal([]byte(appJson), &unJson); errJU != nil {
						chErr <- errJU
						return
					}

					app := &unJson.Status.IcingaApplication.App

					env := &Environment{
						Name:     app.Environment,
						ID:       Sha1bytes([]byte(app.Environment)),
						NodeName: app.NodeName,
						Icinga2:  Icinga2Info{app.Version, app.ProgramStart, nil},
					}

					if app.EndpointId != "" {
						if unHex, errHD := hex.DecodeString(app.EndpointId); errHD == nil {
							env.Icinga2.EndpointId = unHex
						}
					}

					chEnv <- env
				}
			}
		}
	}
}
