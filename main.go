package main

import (
	"flag"
	"git.icinga.com/icingadb/icingadb-main/config"
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"git.icinga.com/icingadb/icingadb-main/configobject/configsync"
	"git.icinga.com/icingadb/icingadb-main/configobject/history"
	"git.icinga.com/icingadb/icingadb-main/configobject/nullrows"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/actionurl"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/checkcommand"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/checkcommand/checkcommandargument"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/checkcommand/checkcommandcustomvar"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/checkcommand/checkcommandenvvar"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/comment"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/customvar"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/customvar/customvarflat"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/downtime"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/endpoint"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/eventcommand"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/eventcommand/eventcommandargument"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/eventcommand/eventcommandcustomvar"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/eventcommand/eventcommandenvvar"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/host"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/host/hostcustomvar"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/host/hoststate"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/hostgroup"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/hostgroup/hostgroupcustomvar"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/hostgroup/hostgroupmember"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/iconimage"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/notesurl"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/notification"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/notification/notificationcustomvar"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/notification/notificationuser"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/notification/notificationusergroup"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/notificationcommand"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/notificationcommand/notificationcommandargument"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/notificationcommand/notificationcommandcustomvar"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/notificationcommand/notificationcommandenvvar"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/service"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/service/servicecustomvar"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/service/servicestate"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/servicegroup"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/servicegroup/servicegroupcustomvar"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/servicegroup/servicegroupmember"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/timeperiod"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/timeperiod/timeperiodcustomvar"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/timeperiod/timeperiodoverrideexclude"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/timeperiod/timeperiodoverrideinclude"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/timeperiod/timeperiodrange"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/user"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/user/usercustomvar"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/usergroup"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/usergroup/usergroupcustomvar"
	"git.icinga.com/icingadb/icingadb-main/configobject/objecttypes/usergroup/usergroupmember"
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

	level, _ := log.ParseLevel(config.GetLogging().Level)
	log.SetLevel(level)

	redisInfo := config.GetRedisInfo()
	mysqlInfo := config.GetMysqlInfo()

	redisConn := connection.NewRDBWrapper(redisInfo.Host+":"+redisInfo.Port, redisInfo.PoolSize)

	mysqlConn, err := connection.NewDBWrapper(
		mysqlInfo.User+":"+mysqlInfo.Password+"@tcp("+mysqlInfo.Host+":"+mysqlInfo.Port+")/"+mysqlInfo.Database,
		mysqlInfo.MaxOpenConns,
	)
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

	go haInstance.StartHA(chEnv)
	go ha.IcingaHeartbeatListener(redisConn, chEnv, super.ChErr)

	go jsondecoder.DecodePool(super.ChDecode, super.ChErr, 16)

	go nullrows.InsertNullRows(&super)

	startConfigSyncOperators(&super, haInstance)

	statesync.StartStateSync(&super)

	history.StartHistoryWorkers(&super)

	go haInstance.StartEventListener()

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
	objectTypes := []*configobject.ObjectInformation{
		&host.ObjectInformation,
		&hostcustomvar.ObjectInformation,
		&downtime.ObjectInformation,

		&service.ObjectInformation,
		&servicecustomvar.ObjectInformation,
		&servicestate.ObjectInformation,

		&hostgroup.ObjectInformation,
		&hostgroupcustomvar.ObjectInformation,
		&hostgroupmember.ObjectInformation,

		&servicegroup.ObjectInformation,
		&servicegroupcustomvar.ObjectInformation,
		&servicegroupmember.ObjectInformation,

		&user.ObjectInformation,
		&usercustomvar.ObjectInformation,

		&usergroup.ObjectInformation,
		&usergroupcustomvar.ObjectInformation,
		&usergroupmember.ObjectInformation,

		&notification.ObjectInformation,
		&notificationcustomvar.ObjectInformation,
		&notificationuser.ObjectInformation,
		&notificationusergroup.ObjectInformation,

		&customvar.ObjectInformation,
		&customvarflat.ObjectInformation,

		&zone.ObjectInformation,

		&endpoint.ObjectInformation,

		&actionurl.ObjectInformation,
		&notesurl.ObjectInformation,
		&iconimage.ObjectInformation,

		&timeperiod.ObjectInformation,
		&timeperiodcustomvar.ObjectInformation,
		&timeperiodoverrideinclude.ObjectInformation,
		&timeperiodoverrideexclude.ObjectInformation,
		&timeperiodrange.ObjectInformation,

		&checkcommand.ObjectInformation,
		&checkcommandcustomvar.ObjectInformation,
		&checkcommandargument.ObjectInformation,
		&checkcommandenvvar.ObjectInformation,

		&eventcommand.ObjectInformation,
		&eventcommandcustomvar.ObjectInformation,
		&eventcommandargument.ObjectInformation,
		&eventcommandenvvar.ObjectInformation,

		&notificationcommand.ObjectInformation,
		&notificationcommandcustomvar.ObjectInformation,
		&notificationcommandargument.ObjectInformation,
		&notificationcommandenvvar.ObjectInformation,

		&comment.ObjectInformation,
		&hoststate.ObjectInformation,
	}

	for _, objectInformation := range objectTypes {
		go func(information *configobject.ObjectInformation) {
			super.ChErr <- configsync.Operator(super, haInstance.RegisterNotificationListener(information.NotificationListenerType), information)
		}(objectInformation)
	}
}
