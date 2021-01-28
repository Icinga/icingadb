package main

import (
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
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

var ido = newDb("IDO")
var icingaDb = newDb("Icinga DB")
var bulk = flag.Int("bulk", 200, "FACTOR")

var icingaEnv, icingaEndpoint, cache stringValue
var envId, endpointId []byte

func main() {
	flag.Var(&icingaEnv, "icinga-env", "ENVIRONMENT")
	flag.Var(&icingaEndpoint, "icinga-endpoint", "ENDPOINT")
	flag.Var(&cache, "cache", "FILE")
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

	ido.connect()
	icingaDb.connect()

	syncStates()
}

// assert logs message with fields and err and terminates the program if err is not nil.
func assert(err error, message string, fields log.Fields) {
	if err != nil {
		log.WithFields(fields).WithFields(log.Fields{"error": err.Error()}).Fatal(message)
	}
}
