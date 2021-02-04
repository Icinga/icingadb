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

var icingaEnv, icingaEndpoint, aHcache, fHcache, nHcache, sHcache stringValue
var envId, endpointId []byte

var cacheBar = newMultiTaskBar(4)
var syncBar = newMultiTaskBar(6)

func main() {
	flag.Var(&icingaEnv, "icinga-env", "ENVIRONMENT")
	flag.Var(&icingaEndpoint, "icinga-endpoint", "ENDPOINT")
	flag.Var(&aHcache, "ah-cache", "FILE")
	flag.Var(&fHcache, "fh-cache", "FILE")
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

	if !aHcache.isSet {
		fmt.Fprintln(os.Stderr, "-ah-cache missing")
		flag.Usage()
		os.Exit(2)
	}

	if !fHcache.isSet {
		fmt.Fprintln(os.Stderr, "-fh-cache missing")
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

	go syncAcks()
	go syncComments()
	go syncDowntimes()
	go syncFlapping()
	go syncNotifications()
	go syncStates()

	cacheBar.runMaster()

	log.Info("Migrating history")
	syncBar.runMaster()
}
