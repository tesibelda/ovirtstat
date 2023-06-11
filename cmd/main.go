// ovirtstat main package is a telegraf shim that allows ovirtstat to work as an execd input
//  plugin so you can monitor oVirt status and basic stats
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/influxdata/telegraf/plugins/common/shim"

	"github.com/tesibelda/ovirtstat/plugins/inputs/ovirtstat"
)

// Version cotains the actual version of ovirtstat
var Version string

func main() {
	var (
		pollInterval = flag.Duration(
			"poll_interval",
			60*time.Second,
			"how often to send metrics (default 1m)",
		)
		configFile  = flag.String("config", "", "path to the config file for this plugin")
		showVersion = flag.Bool("version", false, "show ovirtstat version and exit")
		err         error
	)

	// parse command line options
	flag.Parse()
	if *showVersion {
		fmt.Println("ovirtstat", Version)
		os.Exit(0)
	}

	// create the shim. This is what will run your plugins.
	shim := shim.New()
	if shim == nil {
		fmt.Fprintf(os.Stderr, "Error creating telegraf shim\n")
		os.Exit(1)
	}

	// If no config is specified, all imported plugins are loaded.
	// otherwise follow what the config asks for.
	// Check for settings from a config toml file,
	// (or just use whatever plugins were imported above)
	if err = shim.LoadConfig(configFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %s\n", err)
		os.Exit(1)
	}

	// Tell ovirtstat shim the configured polling interval
	vcCfg, ok := shim.Input.(*ovirtstat.Config)
	if !ok {
		fmt.Fprintf(os.Stderr, "Error getting shim input as ovirtstat Config\n")
		os.Exit(1)
	}
	if err = vcCfg.SetPollInterval(*pollInterval); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting ovirtstat shim polling interval: %s\n", err)
		os.Exit(1)
	}

	// run a single plugin until stdin closes or we receive a termination signal
	if err = shim.Run(*pollInterval); err != nil {
		fmt.Fprintf(os.Stderr, "Error running telegraf shim: %s\n", err)
		os.Exit(2)
	}
}
