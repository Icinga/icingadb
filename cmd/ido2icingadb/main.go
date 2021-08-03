package main

import (
	"github.com/icinga/icingadb/cmd/internal"
	"github.com/jessevdk/go-flags"
	"os"
)

// Flags defines the CLI flags.
type Flags struct {
	// Config is the path to the config file.
	Config string `short:"c" long:"config" description:"path to config file" required:"true"`
}

func main() {
	os.Exit(run())
}

func run() int {
	f := &Flags{}
	if _, err := flags.NewParser(f, flags.Default).Parse(); err != nil {
		return 2
	}

	// TODO
	return internal.ExitSuccess
}
