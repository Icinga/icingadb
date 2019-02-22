package icingadb_ha_lib

import (
	"crypto/sha1"
	"encoding/json"
	"git.icinga.com/icingadb-connection"
	log "github.com/sirupsen/logrus"
	)

// Compute SHA1
func Sha1bytes(bytes []byte) []byte {
	hash := sha1.New()
	hash.Write(bytes)
	return hash.Sum(nil)
}


func IcingaEventsBroker(rdb *icingadb_connection.RDBWrapper, chEnv chan *icingadb_connection.Environment) error {
	log.Info("Starting Events broker")

	subscription := rdb.Rdb.Subscribe()
	defer subscription.Close();


	if err := subscription.Subscribe(
		"icinga:config:dump", "icinga:config:delete", "icinga:config:update", "icinga:stats"); err != nil {
		return err
	}

	for {
		msg, err := subscription.ReceiveMessage()
		if err != nil {
			return err
		}

		switch msg.Channel {
		case "icinga:stats":
			var unJson interface{} = nil
			if err = json.Unmarshal([]byte(msg.Payload), &unJson); err != nil {
				return err
			}

			environment := unJson.(map[string]interface{})["IcingaApplication"].(map[string]interface{})["status"].(map[string]interface{})["icingaapplication"].(map[string]interface{})["app"].(map[string]interface{})["environment"].(string)
			env := &icingadb_connection.Environment{Name: environment, ID: Sha1bytes([]byte(environment))}
			chEnv <- env
		}
	}
}
