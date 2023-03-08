// This file contains ovirtcollector methods to gathers stats about hosts
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package ovirtcollector

import (
	"context"
	"fmt"
	"time"

	"github.com/influxdata/telegraf"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

// CollectHostInfo gathers oVirt host's info
func (c *OVirtCollector) CollectHostInfo(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		status              ovirtsdk.HostStatus
		htype               ovirtsdk.HostType
		cl                  *ovirtsdk.Cluster
		cpu                 *ovirtsdk.Cpu
		cort                *ovirtsdk.CpuTopology
		vmsumm              *ovirtsdk.VmSummary
		hotags              = make(map[string]string)
		hofields            = make(map[string]interface{})
		id, name, dcname    string
		clname              string
		t                   time.Time
		mem, cores          int64
		sockets, threads    int64
		vmact, vmmig, vmtot int64
		speed               float64
		ok, reinstall       bool
		err                 error
	)

	if c.conn == nil {
		return fmt.Errorf("Could not get hosts info: %w", ErrorNoClient)
	}

	if err = c.getAllDatacentersHosts(ctx); err != nil {
		return fmt.Errorf("Could not get all hosts entity lists: %w", err)
	}
	t = time.Now()

	for _, host := range c.hosts.Slice() {
		if id, ok = host.Id(); !ok {
			acc.AddError(fmt.Errorf("Found a host without Id, skipping"))
			continue
		}
		if name, ok = host.Name(); !ok {
			acc.AddError(fmt.Errorf("Found a host without Name, skipping"))
			continue
		}
		if !c.filterHosts.Match(name) {
			continue
		}
		htype, _ = host.Type()
		clname, dcname = "", ""
		if cl, ok = host.Cluster(); ok {
			clname = c.clusterName(cl)
			if !c.filterClusters.Match(clname) {
				continue
			}
			dcname = c.clusterDatacenterName(cl)
		}
		if status, ok = host.Status(); !ok {
			acc.AddError(fmt.Errorf("Cloud not get status for host %s", name))
			continue
		}
		cores, sockets, speed, threads = 0, 0, 0, 0
		if cpu, ok = host.Cpu(); ok {
			if cort, ok = cpu.Topology(); ok {
				cores, _ = cort.Cores()
				sockets, _ = cort.Sockets()
				threads, _ = cort.Threads()
			}
			speed, _ = cpu.Speed()
		}
		if mem, ok = host.Memory(); !ok {
			mem = 0
		}
		if reinstall, ok = host.ReinstallationRequired(); !ok {
			reinstall = false
		}
		vmact, vmmig, vmtot = 0, 0, 0
		if vmsumm, ok = host.Summary(); ok {
			vmact, _ = vmsumm.Active()
			vmmig, _ = vmsumm.Migrating()
			vmtot, _ = vmsumm.Total()
		}

		hotags["clustername"] = clname
		hotags["dcname"] = dcname
		hotags["id"] = id
		hotags["name"] = name
		hotags["ovirt-engine"] = c.url.Host
		hotags["type"] = string(htype)

		hofields["cpu_cores"] = cores
		hofields["cpu_sockets"] = sockets
		hofields["cpu_speed"] = speed
		hofields["cpu_threads"] = threads
		hofields["memory_size"] = mem
		hofields["reinstallation_required"] = reinstall
		hofields["status"] = string(status)
		hofields["status_code"] = hostStatusCode(status)
		hofields["vm_active"] = vmact
		hofields["vm_migrating"] = vmmig
		hofields["vm_total"] = vmtot

		acc.AddFields("ovirtstat_host", hofields, hotags, t)
	}

	return err
}

// hostStatusCode converts HostStatus to int16 for easy alerting
func hostStatusCode(status ovirtsdk.HostStatus) int16 {
	switch status {
	case ovirtsdk.HOSTSTATUS_UP:
		return 0
	case ovirtsdk.HOSTSTATUS_UNASSIGNED:
		return 4
	case ovirtsdk.HOSTSTATUS_REBOOT:
		return 6
	case ovirtsdk.HOSTSTATUS_PREPARING_FOR_MAINTENANCE:
		return 2
	case ovirtsdk.HOSTSTATUS_PENDING_APPROVAL:
		return 3
	case ovirtsdk.HOSTSTATUS_NON_RESPONSIVE:
		return 10
	case ovirtsdk.HOSTSTATUS_NON_OPERATIONAL:
		return 11
	case ovirtsdk.HOSTSTATUS_MAINTENANCE:
		return 1
	case ovirtsdk.HOSTSTATUS_KDUMPING:
		return 8
	case ovirtsdk.HOSTSTATUS_INSTALLING_OS:
		return 5
	case ovirtsdk.HOSTSTATUS_INSTALLING:
		return 5
	case ovirtsdk.HOSTSTATUS_INSTALL_FAILED:
		return 5
	case ovirtsdk.HOSTSTATUS_INITIALIZING:
		return 6
	case ovirtsdk.HOSTSTATUS_ERROR:
		return 9
	case ovirtsdk.HOSTSTATUS_DOWN:
		return 12
	case ovirtsdk.HOSTSTATUS_CONNECTING:
		return 7
	default:
		return 1
	}
}
