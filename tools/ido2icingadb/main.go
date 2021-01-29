package main

import (
	"flag"
	"fmt"
	"github.com/cheggaaa/pb/v3"
	log "github.com/sirupsen/logrus"
	"os"
	"sync"
)

// stringValue allows to differ a string not passed via the CLI and an empty string passed via the CLI
// w/o polluting the usage instructions.
type stringValue struct {
	// value is the string passed via the CLI if any.
	value string
	// isSet tells whether the string was passed.
	isSet bool
}

var _ flag.Value = (*stringValue)(nil)

// String implements flag.Value.
func (sv *stringValue) String() string {
	return sv.value
}

// Set implements flag.Value.
func (sv *stringValue) Set(s string) error {
	sv.value = s
	sv.isSet = true
	return nil
}

// multiTaskBar lets multiple workers report their progress to a single progress bar.
type multiTaskBar struct {
	// items contains the amount of work per worker.
	items chan int
	// bar indicates the overall progress.
	bar *pb.ProgressBar
	// start indicates that bar is ready.
	start chan struct{}
	// wg indicates that the workers are done.
	wg sync.WaitGroup
}

// runMaster coordinates everything and waits until the workers are done.
func (mtb *multiTaskBar) runMaster() {
	items := 0
	for i := cap(mtb.items); i > 0; i-- {
		items += <-mtb.items
	}

	mtb.bar = pb.StartNew(items)
	close(mtb.start)

	mtb.wg.Wait()
	mtb.bar.Finish()
}

// startWorker shall be called once per worker with their individual amount of work.
func (mtb *multiTaskBar) startWorker(items int) *pb.ProgressBar {
	mtb.items <- items
	<-mtb.start
	return mtb.bar
}

// stopWorker shall be called once per worker once done.
func (mtb *multiTaskBar) stopWorker() {
	mtb.wg.Done()
}

// newMultiTaskBar creates a new multiTaskBar suitable for workers workers.
func newMultiTaskBar(workers int) *multiTaskBar {
	mtb := &multiTaskBar{
		items: make(chan int, workers),
		start: make(chan struct{}),
	}

	mtb.wg.Add(workers)
	return mtb
}

var ido = newDb("IDO")
var icingaDb = newDb("Icinga DB")
var bulk = flag.Int("bulk", 200, "FACTOR")

var icingaEnv, icingaEndpoint, nHcache, sHcache stringValue
var envId, endpointId []byte

var cacheBar = newMultiTaskBar(2)
var syncBar = newMultiTaskBar(2)

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

	ido.connect()
	icingaDb.connect()

	log.Info("Building cache")

	go syncNotifications()
	go syncStates()

	cacheBar.runMaster()

	log.Info("Migrating history")
	syncBar.runMaster()
}

// assert logs message with fields and err and terminates the program if err is not nil.
func assert(err error, message string, fields log.Fields) {
	if err != nil {
		log.WithFields(fields).WithFields(log.Fields{"error": err.Error()}).Fatal(message)
	}
}
