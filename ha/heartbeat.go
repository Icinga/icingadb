// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package ha

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/utils"
	"github.com/go-redis/redis/v7"
	"github.com/intel-go/fastjson"
	log "github.com/sirupsen/logrus"
	"time"
)

type Environment struct {
	ID       []byte
	Name     string
	NodeName string
	Icinga2  Icinga2Info
}

type Icinga2Info struct {
	Version                    string
	ProgramStart               float64
	EndpointId                 []byte
	NotificationsEnabled       bool
	ActiveServiceChecksEnabled bool
	ActiveHostChecksEnabled    bool
	EventHandlersEnabled       bool
	FlapDetectionEnabled       bool
	PerformanceDataEnabled     bool
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
		Streams: []string{"icinga:stats", fmt.Sprintf("%d-0", utils.TimeToMillisecs(time.Now().Add(-15*time.Second)))},
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
									Environment                string  `json:"environment"`
									NodeName                   string  `json:"node_name"`
									Version                    string  `json:"version"`
									ProgramStart               float64 `json:"program_start"`
									EndpointId                 string  `json:"endpoint_id"`
									NotificationsEnabled       bool    `json:"enable_notifications"`
									ActiveServiceChecksEnabled bool    `json:"enable_service_checks"`
									ActiveHostChecksEnabled    bool    `json:"enable_host_checks"`
									EventHandlersEnabled       bool    `json:"enable_event_handlers"`
									FlapDetectionEnabled       bool    `json:"enable_flapping"`
									PerformanceDataEnabled     bool    `json:"enable_perfdata"`
								} `json:"app"`
							} `json:"icingaapplication"`
						} `json:"status"`
					}

					if errJU := fastjson.Unmarshal([]byte(appJson), &unJson); errJU != nil {
						chErr <- errJU
						return
					}

					app := &unJson.Status.IcingaApplication.App

					env := &Environment{
						Name:     app.Environment,
						ID:       Sha1bytes([]byte(app.Environment)),
						NodeName: app.NodeName,
						Icinga2: Icinga2Info{
							app.Version,
							app.ProgramStart,
							nil,
							app.NotificationsEnabled,
							app.ActiveServiceChecksEnabled,
							app.ActiveHostChecksEnabled,
							app.EventHandlersEnabled,
							app.FlapDetectionEnabled,
							app.PerformanceDataEnabled,
						},
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
