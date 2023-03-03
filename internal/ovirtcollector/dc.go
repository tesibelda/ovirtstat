// This file contains ovirtcollector methods to gathers stats about datacenters
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

// CollectDatacenterInfo gathers oVirt datacenter's info
func (c *OVirtCollector) CollectDatacenterInfo(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		status   ovirtsdk.DataCenterStatus
		dctags   = make(map[string]string)
		dcfields = make(map[string]interface{})
		id, name string
		t        time.Time
		ok       bool
		err      error
	)

	if c.conn == nil {
		return fmt.Errorf("Could not get datacenters info: %w", ErrorNoClient)
	}

	if err = c.getDatacentersAndClusters(ctx); err != nil {
		return fmt.Errorf("Could not get all datacenter entity lists: %w", err)
	}
	t = time.Now()

	for _, dc := range c.dcs.Slice() {
		if id, ok = dc.Id(); !ok {
			acc.AddError(fmt.Errorf("Found a datacenter without Id, skipping"))
			continue
		}
		if name, ok = dc.Name(); !ok {
			acc.AddError(fmt.Errorf("Found a datacenter without Name, skipping"))
			continue
		}
		if status, ok = dc.Status(); !ok {
			acc.AddError(fmt.Errorf("Cloud not get status for datacenter %s", name))
			continue
		}

		dctags["name"] = name
		dctags["id"] = id
		dctags["ovirt-engine"] = c.url.Host

		dcfields["clusters"] = c.countClustersInDc(id)
		dcfields["status"] = string(status)
		dcfields["status_code"] = datacenterStatusCode(status)

		acc.AddFields("ovirtstat_datacenter", dcfields, dctags, t)
	}

	return err
}

// datacenterStatusCode converts DataCenterStatus to int16 for easy alerting
func datacenterStatusCode(status ovirtsdk.DataCenterStatus) int16 {
	switch status {
	case ovirtsdk.DATACENTERSTATUS_UP:
		return 0
	case ovirtsdk.DATACENTERSTATUS_UNINITIALIZED:
		return 2
	case ovirtsdk.DATACENTERSTATUS_PROBLEMATIC:
		return 3
	case ovirtsdk.DATACENTERSTATUS_NOT_OPERATIONAL:
		return 5
	case ovirtsdk.DATACENTERSTATUS_MAINTENANCE:
		return 1
	case ovirtsdk.DATACENTERSTATUS_CONTEND:
		return 4
	default:
		return 1
	}
}
