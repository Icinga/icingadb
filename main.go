// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package main

import (
	"flag"
	"fmt"
	"github.com/Icinga/icingadb/config"
	"github.com/Icinga/icingadb/configobject"
	"github.com/Icinga/icingadb/configobject/configsync"
	"github.com/Icinga/icingadb/configobject/history"
	"github.com/Icinga/icingadb/configobject/objecttypes/actionurl"
	"github.com/Icinga/icingadb/configobject/objecttypes/checkcommand"
	"github.com/Icinga/icingadb/configobject/objecttypes/checkcommand/checkcommandargument"
	"github.com/Icinga/icingadb/configobject/objecttypes/checkcommand/checkcommandcustomvar"
	"github.com/Icinga/icingadb/configobject/objecttypes/checkcommand/checkcommandenvvar"
	"github.com/Icinga/icingadb/configobject/objecttypes/comment"
	"github.com/Icinga/icingadb/configobject/objecttypes/customvar"
	"github.com/Icinga/icingadb/configobject/objecttypes/customvar/customvarflat"
	"github.com/Icinga/icingadb/configobject/objecttypes/downtime"
	"github.com/Icinga/icingadb/configobject/objecttypes/endpoint"
	"github.com/Icinga/icingadb/configobject/objecttypes/eventcommand"
	"github.com/Icinga/icingadb/configobject/objecttypes/eventcommand/eventcommandargument"
	"github.com/Icinga/icingadb/configobject/objecttypes/eventcommand/eventcommandcustomvar"
	"github.com/Icinga/icingadb/configobject/objecttypes/eventcommand/eventcommandenvvar"
	"github.com/Icinga/icingadb/configobject/objecttypes/host"
	"github.com/Icinga/icingadb/configobject/objecttypes/host/hostcustomvar"
	"github.com/Icinga/icingadb/configobject/objecttypes/host/hoststate"
	"github.com/Icinga/icingadb/configobject/objecttypes/hostgroup"
	"github.com/Icinga/icingadb/configobject/objecttypes/hostgroup/hostgroupcustomvar"
	"github.com/Icinga/icingadb/configobject/objecttypes/hostgroup/hostgroupmember"
	"github.com/Icinga/icingadb/configobject/objecttypes/iconimage"
	"github.com/Icinga/icingadb/configobject/objecttypes/notesurl"
	"github.com/Icinga/icingadb/configobject/objecttypes/notification"
	"github.com/Icinga/icingadb/configobject/objecttypes/notification/notificationcustomvar"
	"github.com/Icinga/icingadb/configobject/objecttypes/notification/notificationrecipient"
	"github.com/Icinga/icingadb/configobject/objecttypes/notification/notificationuser"
	"github.com/Icinga/icingadb/configobject/objecttypes/notification/notificationusergroup"
	"github.com/Icinga/icingadb/configobject/objecttypes/notificationcommand"
	"github.com/Icinga/icingadb/configobject/objecttypes/notificationcommand/notificationcommandargument"
	"github.com/Icinga/icingadb/configobject/objecttypes/notificationcommand/notificationcommandcustomvar"
	"github.com/Icinga/icingadb/configobject/objecttypes/notificationcommand/notificationcommandenvvar"
	"github.com/Icinga/icingadb/configobject/objecttypes/service"
	"github.com/Icinga/icingadb/configobject/objecttypes/service/servicecustomvar"
	"github.com/Icinga/icingadb/configobject/objecttypes/service/servicestate"
	"github.com/Icinga/icingadb/configobject/objecttypes/servicegroup"
	"github.com/Icinga/icingadb/configobject/objecttypes/servicegroup/servicegroupcustomvar"
	"github.com/Icinga/icingadb/configobject/objecttypes/servicegroup/servicegroupmember"
	"github.com/Icinga/icingadb/configobject/objecttypes/timeperiod"
	"github.com/Icinga/icingadb/configobject/objecttypes/timeperiod/timeperiodcustomvar"
	"github.com/Icinga/icingadb/configobject/objecttypes/timeperiod/timeperiodoverrideexclude"
	"github.com/Icinga/icingadb/configobject/objecttypes/timeperiod/timeperiodoverrideinclude"
	"github.com/Icinga/icingadb/configobject/objecttypes/timeperiod/timeperiodrange"
	"github.com/Icinga/icingadb/configobject/objecttypes/user"
	"github.com/Icinga/icingadb/configobject/objecttypes/user/usercustomvar"
	"github.com/Icinga/icingadb/configobject/objecttypes/usergroup"
	"github.com/Icinga/icingadb/configobject/objecttypes/usergroup/usergroupcustomvar"
	"github.com/Icinga/icingadb/configobject/objecttypes/usergroup/usergroupmember"
	"github.com/Icinga/icingadb/configobject/objecttypes/zone"
	"github.com/Icinga/icingadb/configobject/statesync"
	"github.com/Icinga/icingadb/connection"
	"github.com/Icinga/icingadb/ha"
	"github.com/Icinga/icingadb/jsondecoder"
	"github.com/Icinga/icingadb/prometheus"
	"github.com/Icinga/icingadb/supervisor"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"regexp"
	"sync"
	"syscall"
)

var gitVersion = regexp.MustCompile(`\A(.+)-\d+-g([A-Fa-f0-9]+)\z`)

func main() {
	{
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
		go handleSignal(ch)
	}

	configPath := flag.String("config", "icingadb.ini", "path to config")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		if match := gitVersion.FindStringSubmatch(version); match == nil {
			fmt.Printf("Icinga DB version: %s\n", version)
		} else {
			fmt.Printf("Icinga DB version:%s build:%s\n", match[1], match[2])
		}
		return
	}

	if err := config.ParseConfig(*configPath); err != nil {
		log.Fatalf("Error reading config: %v", err)
	}

	level, _ := log.ParseLevel(config.GetLogging().Level)
	log.SetLevel(level)

	redisInfo := config.GetRedisInfo()
	dbDriver, dbInfo := config.GetDbInfo()
	metricsInfo := config.GetMetricsInfo()

	redisConn := connection.NewRDBWrapper(redisInfo.Host+":"+redisInfo.Port, redisInfo.PoolSize)

	dbConn, err := connection.NewDBWrapper(dbDriver, dbInfo)
	if err != nil {
		log.Fatal(err)
	}

	super := supervisor.Supervisor{
		ChErr:        make(chan error),
		ChDecode:     make(chan *jsondecoder.JsonDecodePackages),
		Rdbw:         redisConn,
		Dbw:          dbConn,
		EnvLock:      &sync.Mutex{},
		WgConfigSync: &sync.WaitGroup{},
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
		go prometheus.HandleHttp("["+metricsInfo.Host+"]:"+metricsInfo.Port, super.ChErr)
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
		&notificationrecipient.ObjectInformation,

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

func handleSignal(ch <-chan os.Signal) {
	if sig, ok := <-ch; ok {
		log.WithFields(log.Fields{"signal": sig}).Info("Shutting down")
		os.Exit(0)
	}
}
