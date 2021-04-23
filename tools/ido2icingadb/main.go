package main

import (
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/vbauerster/mpb/v6"
	"os"
	"sync"
	"time"
)

var ido = newDb("IDO")
var icingaDb = newDb("Icinga DB")
var bulk = flag.Int("bulk", 200, "FACTOR")
var retryAfter = flag.Duration("retry-after", 5*time.Minute, "DURATION")
var chSize = 64

var icingaEnv, icingaEndpoint, cache stringValue
var envId, endpointId []byte

var wg = &sync.WaitGroup{}
var prg = mpb.New()

func main() {
	flag.Var(&icingaEnv, "icinga-env", "ENVIRONMENT")
	flag.Var(&icingaEndpoint, "icinga-endpoint", "ENDPOINT")
	flag.Var(&cache, "cache", "DIRECTORY")
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

	if !cache.isSet {
		fmt.Fprintln(os.Stderr, "-cache missing")
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

	assert(os.MkdirAll(cache.value, 0700), "Couldn't create cache dir", log.Fields{"path": cache.value})

	wg.Add(6)

	go syncAcks()
	go syncComments()
	go syncDowntimes()
	go syncFlapping()
	go syncNotifications()
	go syncStates()

	wg.Wait()
}
