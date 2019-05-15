package main

import (
	"flag"
	"git.icinga.com/icingadb/icingadb-main/config"
	"git.icinga.com/icingadb/icingadb-main/configobject/configsync"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/host"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/hostgroup"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/service"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/servicegroup"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/user"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/zone"
	"git.icinga.com/icingadb/icingadb-main/configobject/statesync"
	"git.icinga.com/icingadb/icingadb-main/connection"
	"git.icinga.com/icingadb/icingadb-main/ha"
	"git.icinga.com/icingadb/icingadb-main/jsondecoder"
	"git.icinga.com/icingadb/icingadb-main/prometheus"
	"git.icinga.com/icingadb/icingadb-main/supervisor"
	log "github.com/sirupsen/logrus"
	"sync"
)

func main() {
	configPath := flag.String("config", "icingadb.ini", "path to config")
	flag.Parse()

	if err := config.ParseConfig(*configPath); err != nil {
		log.Fatalf("Error reading config: %v", err)
	}

	redisInfo := config.GetRedisInfo()
	mysqlInfo := config.GetMysqlInfo()

	redisConn, err := connection.NewRDBWrapper(redisInfo.Host + ":" + redisInfo.Port)
	if err != nil {
		log.Fatal(err)
	}

	mysqlConn, err := connection.NewDBWrapper(mysqlInfo.User + ":" + mysqlInfo.Password + "@tcp(" + mysqlInfo.Host + ":" + mysqlInfo.Port + ")/" + mysqlInfo.Database)
	if err != nil {
		log.Fatal(err)
	}

	super := supervisor.Supervisor{
		ChErr:    make(chan error),
		ChDecode: make(chan *jsondecoder.JsonDecodePackages),
		Rdbw:     redisConn,
		Dbw:      mysqlConn,
		EnvLock:  &sync.Mutex{},
	}

	chEnv := make(chan *ha.Environment)
	haInstance, err := ha.NewHA(&super)
	if err != nil {
		log.Fatal(err)
	}

	go haInstance.Run(chEnv)
	go func() {
		super.ChErr <- ha.IcingaEventsBroker(redisConn, chEnv)
	}()

	go jsondecoder.DecodePool(super.ChDecode, super.ChErr, 16)

	startConfigSyncOperators(&super, haInstance)

	statesync.StartStateSync(&super)

	go prometheus.HandleHttp("0.0.0.0:8080", super.ChErr)

	for {
		select {
		case err := <-super.ChErr:
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func startConfigSyncOperators(super *supervisor.Supervisor, haInstance *ha.HA) {
	chHAHost := haInstance.RegisterNotificationListener()
	go func() {
		super.ChErr <- configsync.Operator(super, chHAHost, &host.ObjectInformation)
	}()

	chHAService := haInstance.RegisterNotificationListener()
	go func() {
		super.ChErr <- configsync.Operator(super, chHAService, &service.ObjectInformation)
	}()

	chHAHostgroup := haInstance.RegisterNotificationListener()
	go func() {
		super.ChErr <- configsync.Operator(super, chHAHostgroup, &hostgroup.ObjectInformation)
	}()

	chHAServicegroup := haInstance.RegisterNotificationListener()
	go func() {
		super.ChErr <- configsync.Operator(super, chHAServicegroup, &servicegroup.ObjectInformation)
	}()

	chHAUser := haInstance.RegisterNotificationListener()
	go func() {
		super.ChErr <- configsync.Operator(super, chHAUser, &user.ObjectInformation)
	}()

	chHAZone := haInstance.RegisterNotificationListener()
	go func() {
		super.ChErr <- configsync.Operator(super, chHAZone, &zone.ObjectInformation)
	}()
}
