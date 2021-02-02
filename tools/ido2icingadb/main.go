package main

import (
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
)

var ido = newDb("IDO")
var icingaDb = newDb("Icinga DB")
var bulk = flag.Int("bulk", 200, "FACTOR")
var chSize = 64

var icingaEnv, icingaEndpoint, nHcache, sHcache stringValue
var envId, endpointId []byte

var cacheBar = newMultiTaskBar(2)
var syncBar = newMultiTaskBar(3)

func main() {
	flag.Var(&icingaEnv, "icinga-env", "ENVIRONMENT")
	flag.Var(&icingaEndpoint, "icinga-endpoint", "ENDPOINT")
	flag.Var(&nHcache, "nh-cache", "FILE")
	flag.Var(&sHcache, "sh-cache", "FILE")
	flag.Parse()

	ido.validate()
	icingaDb.validate()

	if !icingaEnv.isSet {
		fmt.Fprintln(os.Stderr, "-icinga-env missing")
		flag.Usage()
		os.Exit(2)
	}

	if !icingaEndpoint.isSet {
		fmt.Fprintln(os.Stderr, "-icinga-endpoint missing")
		flag.Usage()
		os.Exit(2)
	}

	if !nHcache.isSet {
		fmt.Fprintln(os.Stderr, "-nh-cache missing")
		flag.Usage()
		os.Exit(2)
	}

	if !sHcache.isSet {
		fmt.Fprintln(os.Stderr, "-sh-cache missing")
		flag.Usage()
		os.Exit(2)
	}

	envId = hashStr(icingaEnv.value)
	endpointId = hashAny([2]string{icingaEnv.value, icingaEndpoint.value})

	if *bulk > chSize {
		chSize = *bulk
	}

	ido.connect()
	icingaDb.connect()

	log.Info("Building cache")

	go syncDowntimes()
	go syncNotifications()
	go syncStates()

	cacheBar.runMaster()

	log.Info("Migrating history")
	syncBar.runMaster()
}
