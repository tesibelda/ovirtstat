// ovirtstat is an oVirt input plugin for Telegraf that gathers status and basic
//  stats from oVirt Engine
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package ovirtstat

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/influxdata/telegraf/filter"
	"github.com/influxdata/telegraf/plugins/common/tls"
	"github.com/tesibelda/lightmetric/metric"
	"github.com/tesibelda/ovirtstat/internal/ovirtcollector"
)

type Config struct {
	tls.ClientConfig
	OVirtURL      string        `toml:"ovirturl"`
	Username      string        `toml:"username"`
	Password      string        `toml:"password"`
	Timeout       time.Duration `toml:"timeout"`
	InternalAlias string        `toml:"internal_alias"`

	ClustersExclude []string `toml:"clusters_exclude"`
	ClustersInclude []string `toml:"clusters_include"`
	HostsExclude    []string `toml:"hosts_exclude"`
	HostsInclude    []string `toml:"hosts_include"`
	VmsExclude      []string `toml:"vms_exclude"`
	VmsInclude      []string `toml:"vms_include"`

	CollectorsExclude []string `toml:"collectors_exclude"`
	CollectorsInclude []string `toml:"collectors_include"`
	collectors        map[string]bool
	filterCollectors  filter.Filter

	version      string
	pollInterval time.Duration
	ovc          *ovirtcollector.OVirtCollector

	selfMon     metric.Metric
	gotAnAnswer bool
}

var sampleConfig = `
## OVirt Engine URL to be monitored and its credential
ovirturl = "https://ovirt-engine.local/ovirt-engine/api"
username = "user@internal"
password = "secret"
timeout = "10s"

## Optional SSL Config
# tls_ca = "/path/to/cafile"
## Use SSL but skip chain & host verification
# insecure_skip_verify = false

## optional alias tag for internal metrics
# internal_alias = ""

## Filter clusters by name, default is no filtering
## cluster names can be specified as glob patterns
# clusters_include = []
# clusters_exclude = []

## Filter hosts by name, default is no filtering
## host names can be specified as glob patterns
# hosts_include = []
# hosts_exclude = []

## Filter VMs by name, default is no filtering
## VM names can be specified as glob patterns
# vms_include = []
# vms_exclude = []

## Filter collectors by name, default is all collectors
## see possible collector names bellow
# collectors_include = []
# collectors_exclude = []

#### collector names available are ####
## Datacenters: datacenter stats in ovirtstat_datacenter measurement
## GlusterVolumes: gluster volume stats in ovirtstat_glustervolume measurement
## Hosts: hypervisor/host stats in ovirtstat_host measurement
## StorageDomains: cluster stats in ovirtstat_storagedomains measurement
## VMs: virtual machine stats in ovirtstat_vm measurement
`

func New() *Config {
	return &Config{
		OVirtURL:      "https://ovirt-engine.local/ovirt-engine/api",
		Username:      "user@internal",
		Password:      "secret",
		InternalAlias: "",
		Timeout:       10 * time.Second,
		pollInterval:  time.Second * 60,
	}
}

// LoadConfig reads configuration and initializes internal variables
func (c *Config) LoadConfig(filename string) error {
	if _, err := toml.DecodeFile(filename, &c); err != nil {
		return err
	}

	// expand environment variables for user and pass
	c.Username = expandVar(c.Username)
	c.Password = expandVar(c.Password)

	return nil
}

// Start initializes internal ovirtstat variables with the provided configuration
func (c *Config) Start() error {
	var (
		tags map[string]string
		u    *url.URL
		t    time.Time
		err  error
	)

	if c.ovc != nil {
		c.ovc.Close()
	}
	if c.ovc, err = ovirtcollector.New(
		c.OVirtURL,
		c.Username,
		c.Password,
		&c.ClientConfig,
		c.pollInterval,
	); err != nil {
		return err
	}

	/// Set ovirtcollector options
	c.ovc.SetDataDuration(time.Duration(c.pollInterval.Seconds() * 0.9))
	if err = c.ovc.SetFilterClusters(c.ClustersInclude, c.ClustersExclude); err != nil {
		return fmt.Errorf("error parsing clusters filters: %w", err)
	}
	if err = c.ovc.SetFilterHosts(c.HostsInclude, c.HostsExclude); err != nil {
		return fmt.Errorf("error parsing hosts filters: %w", err)
	}
	if err = c.ovc.SetFilterVms(c.VmsInclude, c.VmsExclude); err != nil {
		return fmt.Errorf("error parsing VMs filters: %w", err)
	}
	if err = c.setFilterCollectors(c.CollectorsInclude, c.CollectorsExclude); err != nil {
		return fmt.Errorf("error parsing collectors filters: %w", err)
	}

	// check OVirt URL
	if u, err = url.Parse(c.OVirtURL); err != nil {
		return fmt.Errorf("error parsing URL for OVirt: %w", err)
	}

	// selfmonitoring
	tags = map[string]string{
		"alias":             c.InternalAlias,
		"ovirt-engine":      u.Hostname(),
		"ovirtstat_version": c.version,
	}
	t = metric.TimeWithPrecision(time.Now(), intervalPrecision(c.pollInterval))
	c.selfMon = metric.New("internal_ovirtstat", tags, nil, t)

	return err
}

// Stop is called from telegraf core when a plugin is stopped and allows it to
// perform shutdown tasks.
func (c *Config) Stop() {
	if c.ovc != nil {
		c.ovc.Close()
		c.ovc = nil
	}
}

// SetPollInterval allows telegraf shim to tell ovirtstat the configured polling interval
func (c *Config) SetPollInterval(pollInterval time.Duration) error {
	c.pollInterval = pollInterval
	return nil
}

// SetVersion lets shim know this version
func (c *Config) SetVersion(version string) {
	c.version = version
}

// SampleConfig returns a set of default configuration to be used as a boilerplate when setting up
// Telegraf.
func (c *Config) SampleConfig() string {
	return sampleConfig
}

// Description returns a short textual description of the plugin
func (c *Config) Description() string {
	return "Gathers status and basic stats from OVirt Engine"
}

// Gather is the main data collection function called by the Telegraf core. It performs all
// the data collection and writes all metrics into the Accumulator passed as an argument.
func (c *Config) Gather(ctx context.Context, acc *metric.Accumulator) error {
	var t, startTime time.Time
	var err error

	startTime = time.Now()
	if err = c.keepActiveSession(ctx, acc); err != nil {
		return gatherError(ctx, err)
	}
	acc.SetPrecision(intervalPrecision(c.pollInterval))

	//--- Get OVirt, DCs and Clusters info
	if err = c.gatherHighLevelEntities(ctx, acc); err != nil {
		return gatherError(ctx, err)
	}

	//--- Get Hosts, Storage and VM info
	if err = c.gatherHost(ctx, acc); err != nil {
		return gatherError(ctx, err)
	}
	if err = c.gatherStorage(ctx, acc); err != nil {
		return gatherError(ctx, err)
	}
	if err = c.gatherVM(ctx, acc); err != nil {
		return gatherError(ctx, err)
	}

	// selfmonitoring
	t = metric.TimeWithPrecision(time.Now(), intervalPrecision(c.pollInterval))
	c.selfMon.SetTime(t)
	c.selfMon.AddField("gather_time_ns", time.Since(startTime).Nanoseconds())
	acc.AddMetric(c.selfMon)

	return nil
}

// keepActiveSession keeps an active session with vsphere
func (c *Config) keepActiveSession(
	ctx context.Context,
	acc *metric.Accumulator,
) error {
	var col *ovirtcollector.OVirtCollector
	var err error

	if ctx.Err() != nil || c.ovc == nil {
		if err = c.Start(); err != nil {
			return fmt.Errorf("failed to initialize collector for %s: %w", c.OVirtURL, err)
		}
	}
	col = c.ovc
	if !col.IsActive(ctx) {
		if c.gotAnAnswer {
			acc.AddError(
				fmt.Errorf("OVirt session not active, re-authenticating with %s", c.OVirtURL),
			)
		}
		if err = col.Open(ctx, c.Timeout); err != nil {
			return fmt.Errorf("failed to open connection with %s: %w", c.OVirtURL, err)
		}

		// selfmonitoring
		c.gotAnAnswer = true
		f, ok := c.selfMon.GetField("sessions_created")
		if ok {
			c.selfMon.AddField("sessions_created", f.(int64)+1)
		} else {
			c.selfMon.AddField("sessions_created", int64(1))
		}
	}

	return nil
}

// gatherHighLevelEntities gathers datacenters and clusters stats
func (c *Config) gatherHighLevelEntities(
	ctx context.Context,
	acc *metric.Accumulator,
) error {
	var col *ovirtcollector.OVirtCollector
	var err error
	var exist bool

	if col = c.ovc; col == nil {
		return ovirtcollector.ErrorNoClient
	}

	//--- Get OVirt api summary stats
	if err = col.CollectAPISummaryInfo(ctx, acc); err != nil {
		return fmt.Errorf("could not to get API summary from %s: %w", c.OVirtURL, err)
	}

	//--- Get Datacenters info
	if _, exist = c.collectors["Datacenters"]; exist {
		err = col.CollectDatacenterInfo(ctx, acc)
	}

	return err
}

// gatherHost gathers info and stats per host
func (c *Config) gatherHost(
	ctx context.Context,
	acc *metric.Accumulator,
) error {
	var col *ovirtcollector.OVirtCollector
	var err error
	var exist bool

	if col = c.ovc; col == nil {
		return ovirtcollector.ErrorNoClient
	}
	if _, exist = c.collectors["Hosts"]; exist {
		err = col.CollectHostInfo(ctx, acc)
	}

	return err
}

// gatherStorage gathers storage entities info
func (c *Config) gatherStorage(
	ctx context.Context,
	acc *metric.Accumulator,
) error {
	var col *ovirtcollector.OVirtCollector
	var err error
	var exist bool

	if col = c.ovc; col == nil {
		return ovirtcollector.ErrorNoClient
	}
	if _, exist = c.collectors["StorageDomains"]; exist {
		err = col.CollectDatastoresInfo(ctx, acc)
	}
	if _, exist = c.collectors["GlusterVolumes"]; exist {
		err = col.CollectGlusterVolumeInfo(ctx, acc)
	}

	return err
}

// gatherVM gathers virtual machine's info
func (c *Config) gatherVM(ctx context.Context, acc *metric.Accumulator) error {
	var col *ovirtcollector.OVirtCollector
	var err error
	var exist bool

	if col = c.ovc; col == nil {
		return ovirtcollector.ErrorNoClient
	}
	if _, exist = c.collectors["VMs"]; exist {
		err = col.CollectVmsInfo(ctx, acc)
	}

	return err
}

// setFilterCollectors sets collectors to use given the include and exclude filters
func (c *Config) setFilterCollectors(include, exclude []string) error {
	var allcollectors = []string{"Datacenters", "GlusterVolumes", "Hosts", "StorageDomains", "VMs"}
	var err error

	c.filterCollectors, err = filter.NewIncludeExcludeFilter(include, exclude)
	if err != nil {
		return err
	}
	if c.collectors == nil {
		c.collectors = make(map[string]bool)
	}
	for _, coll := range allcollectors {
		if c.filterCollectors.Match(coll) {
			c.collectors[coll] = true
		}
	}

	return nil
}

// gatherError adds the error to the metric accumulator
func gatherError(ctx context.Context, err error) error {
	// No need to signal errors if we were merely canceled.
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	return err
}

// intervalPrecision returns the rounding precision for metrics
func intervalPrecision(interval time.Duration) time.Duration {
	switch {
	case interval >= time.Second:
		return time.Second
	case interval >= time.Millisecond:
		return time.Millisecond
	case interval >= time.Microsecond:
		return time.Microsecond
	default:
		return time.Nanosecond
	}
}

func expandVar(vartoexp string) string {
	if strings.HasPrefix(vartoexp, "$") {
		return os.ExpandEnv(vartoexp)
	}
	return vartoexp
}
