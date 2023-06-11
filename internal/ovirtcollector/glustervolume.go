// This file contains ovirtcollector methods to gathers stats about glustervolumes
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

// CollectGlusterVolumeInfo gathers oVirt glustervolume's info
func (c *OVirtCollector) CollectGlusterVolumeInfo(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		status               ovirtsdk.GlusterVolumeStatus
		gvtype               ovirtsdk.GlusterVolumeType
		gvs                  *ovirtsdk.GlusterVolumeSlice
		br                   *ovirtsdk.GlusterBrickSlice
		cl                   *ovirtsdk.Cluster
		gvtags               = make(map[string]string)
		gvfields             = make(map[string]interface{})
		id, name, dcname     string
		clname               string
		t                    time.Time
		briks                int
		disperse, redundancy int64
		replica, stripe      int64
		ok                   bool
		err                  error
	)

	if c.conn == nil {
		return fmt.Errorf("could not get gluster volumes info: %w", ErrorNoClient)
	}

	if err = c.getDatacentersAndClusters(ctx); err != nil {
		return fmt.Errorf("could not get all gluster volumes entity lists: %w", err)
	}
	t = time.Now()

	for _, cl = range c.clusters.Slice() {
		if clname, ok = cl.Name(); !ok {
			acc.AddError(fmt.Errorf("found a cluster without Name, skipping"))
			continue
		}
		if !c.filterClusters.Match(clname) {
			continue
		}
		if gvs, ok = cl.GlusterVolumes(); !ok {
			acc.AddError(fmt.Errorf("could not get gluster volumes for cluster %s", clname))
			continue
		}
		dcname = c.clusterDatacenterName(cl)
		for _, gv := range gvs.Slice() {
			if id, ok = gv.Id(); !ok {
				acc.AddError(fmt.Errorf("found a gluster volume without Id, skipping"))
				continue
			}
			if name, ok = gv.Name(); !ok {
				acc.AddError(fmt.Errorf("vound a gluster volume without Name, skipping"))
				continue
			}
			gvtype, _ = gv.VolumeType()
			if status, ok = gv.Status(); !ok {
				acc.AddError(fmt.Errorf("could not get status for gluster volume %s", name))
				continue
			}
			briks = 0
			if br, ok = gv.Bricks(); ok {
				briks = len(br.Slice())
			}
			disperse, _ = gv.DisperseCount()
			redundancy, _ = gv.RedundancyCount()
			replica, _ = gv.ReplicaCount()
			stripe, _ = gv.StripeCount()

			gvtags["clustername"] = clname
			gvtags["dcname"] = dcname
			gvtags["id"] = id
			gvtags["name"] = name
			gvtags["ovirt-engine"] = c.url.Host
			gvtags["type"] = string(gvtype)

			gvfields["briks"] = briks
			gvfields["disperse_count"] = disperse
			gvfields["redundancy_count"] = redundancy
			gvfields["replica_count"] = replica
			gvfields["status"] = string(status)
			gvfields["status_code"] = gvStatusCode(status)
			gvfields["stripe_count"] = stripe

			acc.AddFields("ovirtstat_glustervolume", gvfields, gvtags, t)
		}
	}

	return err
}

// gvStatusCode converts GlusterVolumeStatus to int16 for easy alerting
func gvStatusCode(status ovirtsdk.GlusterVolumeStatus) int16 {
	switch status {
	case ovirtsdk.GLUSTERVOLUMESTATUS_UP:
		return 0
	case ovirtsdk.GLUSTERVOLUMESTATUS_UNKNOWN:
		return 1
	case ovirtsdk.GLUSTERVOLUMESTATUS_DOWN:
		return 2
	default:
		return 1
	}
}
