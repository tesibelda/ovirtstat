// This file contains ovirtcollector methods to gathers stats about VMs
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

// CollectVmsInfo gathers oVirt VMs info
func (c *OVirtCollector) CollectVmsInfo(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		status           ovirtsdk.VmStatus
		vtype            ovirtsdk.VmType
		cl               *ovirtsdk.Cluster
		ho               *ovirtsdk.Host
		cpu              *ovirtsdk.Cpu
		cort             *ovirtsdk.CpuTopology
		vmtags           = make(map[string]string)
		vmfields         = make(map[string]interface{})
		id, name, dcname string
		clname, hostname string
		t                time.Time
		mem, cores       int64
		sockets, threads int64
		ok, stateless    bool
		runOnce          bool
		err              error
	)

	if c.conn == nil {
		return fmt.Errorf("could not get VMs info: %w", ErrorNoClient)
	}

	if err = c.getAllDatacentersVMs(ctx); err != nil {
		return fmt.Errorf("could not get all VM entity lists: %w", err)
	}
	t = time.Now()

	for _, vm := range c.vms.Slice() {
		if id, ok = vm.Id(); !ok {
			acc.AddError(fmt.Errorf("found a VM without Id, skipping"))
			continue
		}
		if name, ok = vm.Name(); !ok {
			acc.AddError(fmt.Errorf("found a VM without Name, skipping"))
			continue
		}
		if !c.filterVms.Match(name) {
			continue
		}
		if status, ok = vm.Status(); !ok {
			acc.AddError(fmt.Errorf("could not get status for VM %s", name))
			continue
		}
		if ho, ok = vm.Host(); ok {
			hostname = c.hostName(ho)
			if !c.filterHosts.Match(hostname) {
				continue
			}
		}
		vtype, _ = vm.Type()
		clname, dcname = "", ""
		if cl, ok = vm.Cluster(); ok {
			clname = c.clusterName(cl)
			if !c.filterClusters.Match(clname) {
				continue
			}
			dcname = c.clusterDatacenterName(cl)
		}
		cores, sockets, threads = 0, 0, 0
		if cpu, ok = vm.Cpu(); ok {
			if cort, ok = cpu.Topology(); ok {
				cores, _ = cort.Cores()
				sockets, _ = cort.Sockets()
				threads, _ = cort.Threads()
			}
		}
		mem, _ = vm.Memory()
		stateless, _ = vm.Stateless()
		runOnce, _ = vm.RunOnce()

		vmtags["clustername"] = clname
		vmtags["dcname"] = dcname
		vmtags["hostname"] = hostname
		vmtags["id"] = id
		vmtags["name"] = name
		vmtags["ovirt-engine"] = c.url.Host
		vmtags["type"] = string(vtype)

		vmfields["cpu_cores"] = cores
		vmfields["cpu_sockets"] = sockets
		vmfields["cpu_threads"] = threads
		vmfields["memory_size"] = mem
		vmfields["run_once"] = runOnce
		vmfields["stateless"] = stateless
		vmfields["status"] = string(status)
		vmfields["status_code"] = vmStatusCode(status)

		acc.AddFields("ovirtstat_vm", vmfields, vmtags, t)
	}

	return err
}

// vmStatusCode converts VmStatus to int16 for easy alerting
func vmStatusCode(status ovirtsdk.VmStatus) int16 {
	var code int16
	switch status {
	case ovirtsdk.VMSTATUS_UP:
		code = 0
	case ovirtsdk.VMSTATUS_PAUSED:
		code = 1
	case ovirtsdk.VMSTATUS_SUSPENDED:
		code = 2
	case ovirtsdk.VMSTATUS_POWERING_UP:
		code = 3
	case ovirtsdk.VMSTATUS_WAIT_FOR_LAUNCH:
		code = 4
	case ovirtsdk.VMSTATUS_SAVING_STATE:
		code = 5
	case ovirtsdk.VMSTATUS_MIGRATING:
		code = 6
	case ovirtsdk.VMSTATUS_POWERING_DOWN:
		code = 7
	case ovirtsdk.VMSTATUS_RESTORING_STATE:
		code = 9
	case ovirtsdk.VMSTATUS_REBOOT_IN_PROGRESS:
		code = 8
	case ovirtsdk.VMSTATUS_IMAGE_LOCKED:
		code = 11
	case ovirtsdk.VMSTATUS_UNASSIGNED:
		code = 12
	case ovirtsdk.VMSTATUS_NOT_RESPONDING:
		code = 13
	case ovirtsdk.VMSTATUS_DOWN:
		code = 14
	default:
		code = 10
	}
	return code
}
