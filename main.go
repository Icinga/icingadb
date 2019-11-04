package main

import (
	"flag"
	"github.com/icinga/icingadb/config"
	"github.com/icinga/icingadb/configobject"
	"github.com/icinga/icingadb/configobject/configsync"
	"github.com/icinga/icingadb/configobject/history"
	"github.com/icinga/icingadb/configobject/objecttypes/actionurl"
	"github.com/icinga/icingadb/configobject/objecttypes/checkcommand"
	"github.com/icinga/icingadb/configobject/objecttypes/checkcommand/checkcommandargument"
	"github.com/icinga/icingadb/configobject/objecttypes/checkcommand/checkcommandcustomvar"
	"github.com/icinga/icingadb/configobject/objecttypes/checkcommand/checkcommandenvvar"
	"github.com/icinga/icingadb/configobject/objecttypes/comment"
	"github.com/icinga/icingadb/configobject/objecttypes/customvar"
	"github.com/icinga/icingadb/configobject/objecttypes/customvar/customvarflat"
	"github.com/icinga/icingadb/configobject/objecttypes/downtime"
	"github.com/icinga/icingadb/configobject/objecttypes/endpoint"
	"github.com/icinga/icingadb/configobject/objecttypes/eventcommand"
	"github.com/icinga/icingadb/configobject/objecttypes/eventcommand/eventcommandargument"
	"github.com/icinga/icingadb/configobject/objecttypes/eventcommand/eventcommandcustomvar"
	"github.com/icinga/icingadb/configobject/objecttypes/eventcommand/eventcommandenvvar"
	"github.com/icinga/icingadb/configobject/objecttypes/host"
	"github.com/icinga/icingadb/configobject/objecttypes/host/hostcustomvar"
	"github.com/icinga/icingadb/configobject/objecttypes/host/hoststate"
	"github.com/icinga/icingadb/configobject/objecttypes/hostgroup"
	"github.com/icinga/icingadb/configobject/objecttypes/hostgroup/hostgroupcustomvar"
	"github.com/icinga/icingadb/configobject/objecttypes/hostgroup/hostgroupmember"
	"github.com/icinga/icingadb/configobject/objecttypes/iconimage"
	"github.com/icinga/icingadb/configobject/objecttypes/notesurl"
	"github.com/icinga/icingadb/configobject/objecttypes/notification"
	"github.com/icinga/icingadb/configobject/objecttypes/notification/notificationcustomvar"
	"github.com/icinga/icingadb/configobject/objecttypes/notification/notificationuser"
	"github.com/icinga/icingadb/configobject/objecttypes/notification/notificationusergroup"
	"github.com/icinga/icingadb/configobject/objecttypes/notificationcommand"
	"github.com/icinga/icingadb/configobject/objecttypes/notificationcommand/notificationcommandargument"
	"github.com/icinga/icingadb/configobject/objecttypes/notificationcommand/notificationcommandcustomvar"
	"github.com/icinga/icingadb/configobject/objecttypes/notificationcommand/notificationcommandenvvar"
	"github.com/icinga/icingadb/configobject/objecttypes/service"
	"github.com/icinga/icingadb/configobject/objecttypes/service/servicecustomvar"
	"github.com/icinga/icingadb/configobject/objecttypes/service/servicestate"
	"github.com/icinga/icingadb/configobject/objecttypes/servicegroup"
	"github.com/icinga/icingadb/configobject/objecttypes/servicegroup/servicegroupcustomvar"
	"github.com/icinga/icingadb/configobject/objecttypes/servicegroup/servicegroupmember"
	"github.com/icinga/icingadb/configobject/objecttypes/timeperiod"
	"github.com/icinga/icingadb/configobject/objecttypes/timeperiod/timeperiodcustomvar"
	"github.com/icinga/icingadb/configobject/objecttypes/timeperiod/timeperiodoverrideexclude"
	"github.com/icinga/icingadb/configobject/objecttypes/timeperiod/timeperiodoverrideinclude"
	"github.com/icinga/icingadb/configobject/objecttypes/timeperiod/timeperiodrange"
	"github.com/icinga/icingadb/configobject/objecttypes/user"
	"github.com/icinga/icingadb/configobject/objecttypes/user/usercustomvar"
	"github.com/icinga/icingadb/configobject/objecttypes/usergroup"
	"github.com/icinga/icingadb/configobject/objecttypes/usergroup/usergroupcustomvar"
	"github.com/icinga/icingadb/configobject/objecttypes/usergroup/usergroupmember"
	"github.com/icinga/icingadb/configobject/objecttypes/zone"
	"github.com/icinga/icingadb/configobject/statesync"
	"github.com/icinga/icingadb/connection"
	"github.com/icinga/icingadb/ha"
	"github.com/icinga/icingadb/jsondecoder"
	"github.com/icinga/icingadb/prometheus"
	"github.com/icinga/icingadb/supervisor"
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
	metricsInfo := config.GetMetricsInfo()

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

	startConfigSyncOperators(&super, haInstance)

	statesync.StartStateSync(&super)

	history.StartHistoryWorkers(&super)

	go haInstance.StartEventListener()

	if metricsInfo.Host != "" {
		go prometheus.HandleHttp(metricsInfo.Host+":"+metricsInfo.Port, super.ChErr)
	}

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
