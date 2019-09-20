package ha

import (
	"crypto/sha1"
	"encoding/json"
	"git.icinga.com/icingadb/icingadb-main/connection"
	log "github.com/sirupsen/logrus"
)


type Environment struct {
	ID                   []byte
	Name                 string
	configDumpInProgress bool
}

// Compute SHA1
func Sha1bytes(bytes []byte) []byte {
	hash := sha1.New()
	hash.Write(bytes)
	return hash.Sum(nil)
}

func IcingaHeartbeatListener(rdb *connection.RDBWrapper, chEnv chan *Environment) error {
	log.Info("Starting heartbeat listener")

	subscription := rdb.Subscribe()
	defer subscription.Close()
	if err := subscription.Subscribe(
		"icinga:stats"); err != nil {
		return err
	}

	for {
		msg, err := subscription.ReceiveMessage()
		if err != nil {
			return err
		}

		log.Debug("Got heartbeat")

		var unJson interface{} = nil
		if err = json.Unmarshal([]byte(msg.Payload), &unJson); err != nil {
			return err
		}

		environment := unJson.(map[string]interface{})["IcingaApplication"].(map[string]interface{})["status"].(map[string]interface{})["icingaapplication"].(map[string]interface{})["app"].(map[string]interface{})["environment"].(string)
		configDumpInProgress := unJson.(map[string]interface{})["config_dump_in_progress"].(bool)
		env := &Environment{Name: environment, ID: Sha1bytes([]byte(environment)), configDumpInProgress: configDumpInProgress}
		chEnv <- env
	}
}
