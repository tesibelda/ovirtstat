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
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/filter"
	"github.com/influxdata/telegraf/plugins/common/tls"
	"github.com/influxdata/telegraf/plugins/inputs"
	"github.com/influxdata/telegraf/selfstat"

	"github.com/tesibelda/ovirtstat/internal/ovirtcollector"
)

type Config struct {
	tls.ClientConfig
	OVirtURL      string          `toml:"ovirturl"`
	Username      string          `toml:"username"`
	Password      string          `toml:"password"`
	InternalAlias string          `toml:"internal_alias"`
	Log           telegraf.Logger `toml:"-"`

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

	pollInterval time.Duration
	cancel       context.CancelFunc
	ovc          *ovirtcollector.OVirtCollector

	GatherTime      selfstat.Stat
	SessionsCreated selfstat.Stat
}

var sampleConfig = `
  ## OVirt Engine URL to be monitored and its credential
  ovirturl = "https://ovirt-engine.local/ovirt-engine/api"
  username = "user@internal"
  password = "secret"

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

func init() {
	inputs.Add("ovirtstat", func() telegraf.Input {
		return &Config{
			OVirtURL:      "https://ovirt-engine.local/ovirt-engine/api",
			Username:      "user@internal",
			Password:      "secret",
			InternalAlias: "",
			pollInterval:  time.Second * 60,
		}
	})
}

// Init initializes internal ovirtstat variables with the provided configuration
func (ovc *Config) Init() error {
	var err error

	if ovc.ovc != nil {
		ovc.ovc.Close()
	}
	ovc.ovc, err = ovirtcollector.New(
		ovc.OVirtURL,
		ovc.Username,
		ovc.Password,
		&ovc.ClientConfig,
		ovc.pollInterval,
	)
	if err != nil {
		return err
	}

	/// Set ovirtcollector options
	ovc.ovc.SetDataDuration(time.Duration(ovc.pollInterval.Seconds() * 0.9))
	ovc.ovc.SetMaxResponseTime(ovc.pollInterval)
	err = ovc.ovc.SetFilterClusters(ovc.ClustersInclude, ovc.ClustersExclude)
	if err != nil {
		return fmt.Errorf("error parsing clusters filters: %w", err)
	}
	err = ovc.ovc.SetFilterHosts(ovc.HostsInclude, ovc.HostsExclude)
	if err != nil {
		return fmt.Errorf("error parsing hosts filters: %w", err)
	}
	err = ovc.ovc.SetFilterVms(ovc.VmsInclude, ovc.VmsExclude)
	if err != nil {
		return fmt.Errorf("error parsing VMs filters: %w", err)
	}
	err = ovc.setFilterCollectors(ovc.CollectorsInclude, ovc.CollectorsExclude)
	if err != nil {
		return fmt.Errorf("error parsing collectors filters: %w", err)
	}

	// selfmonitoring
	u, err := url.Parse(ovc.OVirtURL)
	if err != nil {
		return fmt.Errorf("error parsing URL for ovurl: %w", err)
	}
	tags := map[string]string{
		"alias":        ovc.InternalAlias,
		"ovirt-engine": u.Hostname(),
	}
	ovc.GatherTime = selfstat.Register("ovirtstat", "gather_time_ns", tags)
	ovc.SessionsCreated = selfstat.Register("ovirtstat", "sessions_created", tags)

	return err
}

// Stop is called from telegraf core when a plugin is stopped and allows it to
// perform shutdown tasks.
func (ovc *Config) Stop() {
	if ovc.ovc != nil {
		ovc.ovc.Close()
		ovc.ovc = nil
	}
	ovc.cancel()
}

// SetPollInterval allows telegraf shim to tell ovirtstat the configured polling interval
func (ovc *Config) SetPollInterval(pollInterval time.Duration) error {
	ovc.pollInterval = pollInterval
	return nil
}

// SampleConfig returns a set of default configuration to be used as a boilerplate when setting up
// Telegraf.
func (ovc *Config) SampleConfig() string {
	return sampleConfig
}

// Description returns a short textual description of the plugin
func (ovc *Config) Description() string {
	return "Gathers status and basic stats from OVirt Engine"
}

// Gather is the main data collection function called by the Telegraf core. It performs all
// the data collection and writes all metrics into the Accumulator passed as an argument.
func (ovc *Config) Gather(acc telegraf.Accumulator) error {
	var startTime time.Time
	var err error

	// poll using a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), ovc.pollInterval)
	defer cancel()

	startTime = time.Now()
	if err = ovc.keepActiveSession(ctx, acc); err != nil {
		return gatherError(ctx, err)
	}
	acc.SetPrecision(intervalPrecision(ovc.pollInterval))

	//--- Get OVirt, DCs and Clusters info
	if err = ovc.gatherHighLevelEntities(ctx, acc); err != nil {
		return gatherError(ctx, err)
	}

	//--- Get Hosts, Storage and VM info
	if err = ovc.gatherHost(ctx, acc); err != nil {
		return gatherError(ctx, err)
	}
	if err = ovc.gatherStorage(ctx, acc); err != nil {
		return gatherError(ctx, err)
	}
	if err = ovc.gatherVM(ctx, acc); err != nil {
		return gatherError(ctx, err)
	}

	// selfmonitoring
	ovc.GatherTime.Set(time.Since(startTime).Nanoseconds())
	for _, m := range selfstat.Metrics() {
		if m.Name() != "internal_agent" {
			acc.AddMetric(m)
		}
	}

	return nil
}

// keepActiveSession keeps an active session with vsphere
func (ovc *Config) keepActiveSession(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var col *ovirtcollector.OVirtCollector
	var err error

	col = ovc.ovc
	if ctx.Err() != nil || col == nil {
		if err = ovc.Init(); err != nil {
			return fmt.Errorf("failed to initialize collector for %s: %w", ovc.OVirtURL, err)
		}
	}
	if !col.IsActive(ctx) {
		if ovc.SessionsCreated.Get() > 0 {
			acc.AddError(
				fmt.Errorf(
					"OVirt session not active, re-authenticating with %s",
					ovc.OVirtURL,
				),
			)
		}
		if err = col.Open(ctx, ovc.pollInterval/4); err != nil {
			return fmt.Errorf("failed to open connection with %s: %w", ovc.OVirtURL, err)
		}
		ovc.SessionsCreated.Incr(1)
	}

	return nil
}

// gatherHighLevelEntities gathers datacenters and clusters stats
func (ovc *Config) gatherHighLevelEntities(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var col *ovirtcollector.OVirtCollector
	var err error
	var exist bool

	if col = ovc.ovc; col == nil {
		return ovirtcollector.ErrorNoClient
	}

	//--- Get OVirt api summary stats
	if err = col.CollectAPISummaryInfo(ctx, acc); err != nil {
		return fmt.Errorf("failed to get API summary for %s: %w", ovc.OVirtURL, err)
	}

	//--- Get Datacenters info
	if _, exist = ovc.collectors["Datacenters"]; exist {
		err = col.CollectDatacenterInfo(ctx, acc)
	}

	return err
}

// gatherHost gathers info and stats per host
func (ovc *Config) gatherHost(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var col *ovirtcollector.OVirtCollector
	var err error
	var exist bool

	if col = ovc.ovc; col == nil {
		return ovirtcollector.ErrorNoClient
	}
	if _, exist = ovc.collectors["Hosts"]; exist {
		err = col.CollectHostInfo(ctx, acc)
	}

	return err
}

// gatherStorage gathers storage entities info
func (ovc *Config) gatherStorage(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var col *ovirtcollector.OVirtCollector
	var err error
	var exist bool

	if col = ovc.ovc; col == nil {
		return ovirtcollector.ErrorNoClient
	}
	if _, exist = ovc.collectors["StorageDomains"]; exist {
		err = col.CollectDatastoresInfo(ctx, acc)
	}
	if _, exist = ovc.collectors["GlusterVolumes"]; exist {
		err = col.CollectGlusterVolumeInfo(ctx, acc)
	}

	return err
}

// gatherVM gathers virtual machine's info
func (ovc *Config) gatherVM(ctx context.Context, acc telegraf.Accumulator) error {
	var col *ovirtcollector.OVirtCollector
	var err error
	var exist bool

	if col = ovc.ovc; col == nil {
		return ovirtcollector.ErrorNoClient
	}
	if _, exist = ovc.collectors["VMs"]; exist {
		err = col.CollectVmsInfo(ctx, acc)
	}

	return err
}

// setFilterCollectors sets collectors to use given the include and exclude filters
func (ovc *Config) setFilterCollectors(include, exclude []string) error {
	var allcollectors = []string{"Datacenters", "GlusterVolumes", "Hosts", "StorageDomains", "VMs"}
	var err error

	ovc.filterCollectors, err = filter.NewIncludeExcludeFilter(include, exclude)
	if err != nil {
		return err
	}
	if ovc.collectors == nil {
		ovc.collectors = make(map[string]bool)
	}
	for _, coll := range allcollectors {
		if ovc.filterCollectors.Match(coll) {
			ovc.collectors[coll] = true
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
