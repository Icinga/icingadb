package ha

import (
	"crypto/sha1"
	"encoding/json"
	"github.com/icinga/icingadb/connection"
	log "github.com/sirupsen/logrus"
)

type Environment struct {
	ID       []byte
	Name     string
	NodeName string
}

// Compute SHA1
func Sha1bytes(bytes []byte) []byte {
	hash := sha1.New()
	hash.Write(bytes)
	return hash.Sum(nil)
}

func IcingaHeartbeatListener(rdb *connection.RDBWrapper, chEnv chan *Environment, chErr chan error) {
	log.Info("Starting heartbeat listener")

	subscription := rdb.Subscribe()
	defer subscription.Close()
	if err := subscription.Subscribe("icinga:stats"); err != nil {
		chErr <- err
		return
	}

	for {
		msg, err := subscription.ReceiveMessage()
		if err != nil {
			chErr <- err
			return
		}

		log.Debug("Got heartbeat")

		var unJson interface{} = nil
		if err = json.Unmarshal([]byte(msg.Payload), &unJson); err != nil {
			chErr <- err
			return
		}

		environment := unJson.(map[string]interface{})["IcingaApplication"].(map[string]interface{})["status"].(map[string]interface{})["icingaapplication"].(map[string]interface{})["app"].(map[string]interface{})["environment"].(string)
		nodeName := unJson.(map[string]interface{})["IcingaApplication"].(map[string]interface{})["status"].(map[string]interface{})["icingaapplication"].(map[string]interface{})["app"].(map[string]interface{})["node_name"].(string)
		env := &Environment{Name: environment, ID: Sha1bytes([]byte(environment)), NodeName: nodeName}
		chEnv <- env
	}
}
