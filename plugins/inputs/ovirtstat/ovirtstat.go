// ovirtstat is an oVirt input plugin for Telegraf that gathers status and basic
//  stats from oVirt Engine
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package ovirtstat

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/filter"
	"github.com/influxdata/telegraf/plugins/common/tls"
	"github.com/influxdata/telegraf/plugins/inputs"
	"github.com/influxdata/telegraf/selfstat"

	"github.com/tesibelda/ovirtstat/internal/ovirtcollector"
	"github.com/tesibelda/vcstat/pkg/tgplus"
)

type OVirtstatConfig struct {
	tls.ClientConfig
	OVirtUrl      string          `toml:"ovirturl"`
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
	ctx          context.Context
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
		return &OVirtstatConfig{
			OVirtUrl:      "https://ovirt-engine.local/ovirt-engine/api",
			Username:      "user@internal",
			Password:      "secret",
			InternalAlias: "",
			pollInterval:  time.Second * 60,
		}
	})
}

// Init initializes internal ovirtstat variables with the provided configuration
func (ovc *OVirtstatConfig) Init() error {
	var err error

	ovc.ctx, ovc.cancel = context.WithCancel(context.Background())
	if ovc.ovc != nil {
		ovc.ovc.Close(ovc.ctx)
	}
	ovc.ovc, err = ovirtcollector.New(
		ovc.ctx,
		ovc.OVirtUrl,
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
	ovc.ovc.SetMaxResponseTime(time.Duration(ovc.pollInterval))
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
	u, err := url.Parse(ovc.OVirtUrl)
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
func (ovc *OVirtstatConfig) Stop() {
	if ovc.ovc != nil {
		ovc.ovc.Close(ovc.ctx)
	}
	ovc.cancel()
}

// SetPollInterval allows telegraf shim to tell ovirtstat the configured polling interval
func (ovc *OVirtstatConfig) SetPollInterval(pollInterval time.Duration) error {
	ovc.pollInterval = pollInterval
	return nil
}

// SampleConfig returns a set of default configuration to be used as a boilerplate when setting up
// Telegraf.
func (ovc *OVirtstatConfig) SampleConfig() string {
	return sampleConfig
}

// Description returns a short textual description of the plugin
func (ovc *OVirtstatConfig) Description() string {
	return "Gathers status and basic stats from OVirt Engine"
}

// Gather is the main data collection function called by the Telegraf core. It performs all
// the data collection and writes all metrics into the Accumulator passed as an argument.
func (ovc *OVirtstatConfig) Gather(acc telegraf.Accumulator) error {
	var startTime time.Time
	var err error

	if err = ovc.keepActiveSession(acc); err != nil {
		return tgplus.GatherError(acc, err)
	}
	acc.SetPrecision(tgplus.GetPrecision(ovc.pollInterval))

	// poll using a context with timeout
	ctxT, cancelT := context.WithTimeout(ovc.ctx, time.Duration(ovc.pollInterval))
	defer cancelT()
	startTime = time.Now()

	//--- Get OVirt, DCs and Clusters info
	if err = ovc.gatherHighLevelEntities(ctxT, acc); err != nil {
		return tgplus.GatherError(acc, err)
	}

	//--- Get Hosts, Storage and VM info
	if err = ovc.gatherHost(ctxT, acc); err != nil {
		return tgplus.GatherError(acc, err)
	}
	if err = ovc.gatherStorage(ctxT, acc); err != nil {
		return tgplus.GatherError(acc, err)
	}
	if err = ovc.gatherVM(ctxT, acc); err != nil {
		return tgplus.GatherError(acc, err)
	}

	// selfmonitoring
	ovc.GatherTime.Set(int64(time.Since(startTime).Nanoseconds()))
	for _, m := range selfstat.Metrics() {
		if m.Name() != "internal_agent" {
			acc.AddMetric(m)
		}
	}

	return nil
}

// keepActiveSession keeps an active session with vsphere
func (ovc *OVirtstatConfig) keepActiveSession(acc telegraf.Accumulator) error {
	var col *ovirtcollector.OVirtCollector
	var err error

	col = ovc.ovc
	if ovc.ctx == nil || ovc.ctx.Err() != nil || col == nil {
		if err = ovc.Init(); err != nil {
			return err
		}
	}
	if !col.IsActive(ovc.ctx) {
		if ovc.SessionsCreated.Get() > 0 {
			acc.AddError(fmt.Errorf("OVirt session not active, re-authenticating"))
		}
		if err = col.Open(ovc.ctx, 12*time.Hour); err != nil {
			return err
		}
		ovc.SessionsCreated.Incr(1)
	}

	return nil
}

// gatherHighLevelEntities gathers datacenters and clusters stats
func (ovc *OVirtstatConfig) gatherHighLevelEntities(
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
	if err = col.CollectApiSummaryInfo(ctx, acc); err != nil {
		return err
	}

	//--- Get Datacenters info
	if _, exist = ovc.collectors["Datacenters"]; exist {
		err = col.CollectDatacenterInfo(ctx, acc)
	}

	return err
}

// gatherHost gathers info and stats per host
func (ovc *OVirtstatConfig) gatherHost(
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
func (ovc *OVirtstatConfig) gatherStorage(
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
func (ovc *OVirtstatConfig) gatherVM(ctx context.Context, acc telegraf.Accumulator) error {
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
func (ovc *OVirtstatConfig) setFilterCollectors(include, exclude []string) error {
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
