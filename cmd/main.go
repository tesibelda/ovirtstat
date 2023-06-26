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

	"github.com/tesibelda/lightmetric/shim"

	"github.com/tesibelda/ovirtstat/plugins/inputs/ovirtstat"
)

const pluginName = "ovirtstat"

// Version cotains the actual version of ovirtstat
var Version string

func main() {
	var (
		pollInterval = flag.Duration(
			"poll_interval",
			60*time.Second,
			"how often to send metrics",
		)
		configFile  = flag.String("config", "", "path to the config file for this plugin")
		showVersion = flag.Bool("version", false, "display ovirtstat version and exit")
		err         error
	)

	// parse command line options
	flag.Parse()
	if *showVersion {
		fmt.Println(pluginName, Version)
		os.Exit(0)
	}
	oV := ovirtstat.New()
	oV.SetVersion(Version)
	for _, col := range flag.Args() {
		switch col {
		case "help":
			help()
			os.Exit(0)
		case "version":
			fmt.Println(pluginName, Version)
			os.Exit(0)
		case "config":
			fmt.Println(oV.SampleConfig())
			os.Exit(0)
		case "run":
		default:
			help()
			os.Exit(1)
		}
	}

	// load config an wait for stdin signal from telegraf to gather data
	if *configFile != "" {
		if err = oV.LoadConfig(*configFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error loading configuration: %s\n", err)
			os.Exit(1)
		}
	}
	if *pollInterval != 0 {
		if err = oV.SetPollInterval(*pollInterval); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting poll interval: %s\n", err)
			os.Exit(1)
		}
	}
	if oV.Timeout > *pollInterval {
		fmt.Fprintf(
			os.Stderr,
			"Timeout cannot be greater than poll_interval so using %s\n",
			*pollInterval,
		)
		oV.Timeout = *pollInterval
	}

	// run a single plugin until stdin closes or we receive a termination signal
	execd := shim.New(pluginName).WithPrecision(time.Second)
	if err = execd.RunInput(oV.Gather); err != nil {
		fmt.Fprintf(os.Stderr, "Error running oVirt Engine collector: %s\n", err)
		os.Exit(2)
	}
	oV.Stop()
}

func help() {
	fmt.Println(pluginName + " [--help] [--config <FILE>] command")
	fmt.Println("COMMANDS:")
	fmt.Println("  help    Display options and commands and exit")
	fmt.Println("  config  Display full sample configuration and exit")
	fmt.Println("  version Display current version and exit")
	fmt.Println("  run     Run as telegraf execd input plugin using signal=stdin. This is the default command.")
}
