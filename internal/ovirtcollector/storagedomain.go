// This file contains ovirtcollector methods to gathers stats about storagedomains
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

// CollectDatastoresInfo gathers oVirt storagedomain's info
func (c *OVirtCollector) CollectDatastoresInfo(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		estatus                    ovirtsdk.ExternalStatus
		status                     ovirtsdk.StorageDomainStatus
		sdty                       ovirtsdk.StorageDomainType
		conns                      *ovirtsdk.StorageConnectionSlice
		sdtags                     = make(map[string]string)
		sdfields                   = make(map[string]interface{})
		id, name, sdtype, stype    string
		t                          time.Time
		available, committed, used int64
		connections, sdlus         int
		ok, master                 bool
		err                        error
	)

	if c.conn == nil {
		return fmt.Errorf("Could not get storagedomains info: %w", ErrorNoClient)
	}

	if err = c.getAllDatacentersStorageDomains(ctx); err != nil {
		return fmt.Errorf("Could not get all storagedomain entity lists: %w", err)
	}
	t = time.Now()

	for _, sd := range c.sds.Slice() {
		if id, ok = sd.Id(); !ok {
			acc.AddError(fmt.Errorf("Found a storagedomain without Id, skipping"))
			continue
		}
		if name, ok = sd.Name(); !ok {
			acc.AddError(fmt.Errorf("Found a storagedomain %s without Name, skipping", id))
			continue
		}
		status, _ = sd.Status() //nolint: external storage may return !ok
		sdtype = ""
		if sdty, ok = sd.Type(); ok {
			sdtype = string(sdty)
		}
		stype, sdlus = getSDStorageInfo(sd)
		used, available, committed = 0, 0, 0
		if status != ovirtsdk.STORAGEDOMAINSTATUS_UNATTACHED {
			if used, ok = sd.Used(); !ok {
				acc.AddError(fmt.Errorf("Cloud not get used for storagedomain %s", name))
				continue
			}
			if available, ok = sd.Available(); !ok {
				acc.AddError(fmt.Errorf("Cloud not get available for storagedomain %s", name))
				continue
			}
			if committed, ok = sd.Committed(); !ok {
				committed = 0
			}
		}
		if master, ok = sd.Master(); !ok {
			master = false
		}
		estatus, _ = sd.ExternalStatus()
		connections = 0
		if conns, ok = sd.StorageConnections(); ok {
			connections = len(conns.Slice())
		}

		sdtags["id"] = id
		sdtags["name"] = name
		sdtags["ovirt-engine"] = c.url.Host
		sdtags["storage_type"] = stype
		sdtags["type"] = sdtype

		sdfields["available"] = available
		sdfields["committed"] = committed
		sdfields["connections"] = connections
		sdfields["external_status"] = string(estatus)
		sdfields["external_status_code"] = externalStatusCode(estatus)
		sdfields["logical_units"] = sdlus
		sdfields["master"] = master
		sdfields["status"] = string(status)
		sdfields["status_code"] = storagedomainStatusCode(status)
		sdfields["used"] = used

		acc.AddFields("ovirtstat_storagedomain", sdfields, sdtags, t)
	}

	return err
}

// getSDStorageInfo returns storage data from a storagedomain
func getSDStorageInfo(sd *ovirtsdk.StorageDomain) (string, int) {
	var (
		sty   ovirtsdk.StorageType
		hst   *ovirtsdk.HostStorage
		lus   *ovirtsdk.LogicalUnitSlice
		vg    *ovirtsdk.VolumeGroup
		stype string
		sdlus int
		ok    bool
	)

	if hst, ok = sd.Storage(); ok {
		if sty, ok = hst.Type(); ok {
			stype = string(sty)
		}
		if lus, ok = hst.LogicalUnits(); ok {
			sdlus = len(lus.Slice())
		} else {
			if vg, ok = hst.VolumeGroup(); ok {
				if lus, ok = vg.LogicalUnits(); ok {
					sdlus = len(lus.Slice())
				}
			}
		}
	}
	return stype, sdlus
}

// storagedomainStatusCode converts StorageDomainStatus to int16 for easy alerting
func storagedomainStatusCode(status ovirtsdk.StorageDomainStatus) int16 {
	switch status {
	case ovirtsdk.STORAGEDOMAINSTATUS_ACTIVATING:
		return 1
	case ovirtsdk.STORAGEDOMAINSTATUS_ACTIVE:
		return 0
	case ovirtsdk.STORAGEDOMAINSTATUS_DETACHING:
		return 4
	case ovirtsdk.STORAGEDOMAINSTATUS_LOCKED:
		return 7
	case ovirtsdk.STORAGEDOMAINSTATUS_MAINTENANCE:
		return 2
	case ovirtsdk.STORAGEDOMAINSTATUS_MIXED:
		return 6
	case ovirtsdk.STORAGEDOMAINSTATUS_PREPARING_FOR_MAINTENANCE:
		return 2
	case ovirtsdk.STORAGEDOMAINSTATUS_UNATTACHED:
		return 5
	case ovirtsdk.STORAGEDOMAINSTATUS_UNKNOWN:
		return 3
	default:
		return 3
	}
}

// externalStatusCode converts ExternalStatus to int16 for easy alerting
func externalStatusCode(status ovirtsdk.ExternalStatus) int16 {
	switch status {
	case ovirtsdk.EXTERNALSTATUS_OK:
		return 0
	case ovirtsdk.EXTERNALSTATUS_INFO:
		return 1
	case ovirtsdk.EXTERNALSTATUS_WARNING:
		return 2
	case ovirtsdk.EXTERNALSTATUS_ERROR:
		return 3
	case ovirtsdk.EXTERNALSTATUS_FAILURE:
		return 4
	default:
		return 1
	}
}
