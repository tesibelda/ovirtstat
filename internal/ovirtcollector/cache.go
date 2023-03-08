// This file contains ovirtcollector methods to cache oVirt entities
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package ovirtcollector

import (
	"context"
	"fmt"
	"time"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

type VcCache struct {
	dcs          *ovirtsdk.DataCenterSlice    //nolint
	clusters     *ovirtsdk.ClusterSlice       //nolint
	sds          *ovirtsdk.StorageDomainSlice //nolint
	hosts        *ovirtsdk.HostSlice          //nolint
	vms          *ovirtsdk.VmSlice            //nolint
	lastDCUpdate time.Time                    //nolint
	lastHoUpdate time.Time                    //nolint
	lastSdUpdate time.Time                    //nolint
	lastVmUpdate time.Time                    //nolint
}

func (c *OVirtCollector) getDatacentersAndClusters(ctx context.Context) error {
	if time.Since(c.lastDCUpdate) < c.dataDuration {
		return nil
	}

	// Get datacenters
	datacentersService := c.conn.SystemService().DataCentersService()
	datacentersResponse, err := datacentersService.List().Send()
	if err != nil {
		return err
	}
	datacenters, ok := datacentersResponse.DataCenters()
	if !ok {
		return fmt.Errorf("Could not get datacenter list or it is empty")
	}
	c.dcs = datacenters
	c.lastDCUpdate = time.Now()

	// Get clusters from datacenter info
	//  dc.Clusters() gives an empty slice, so lets query the full list
	clustersService := c.conn.SystemService().ClustersService()
	clustersResponse, err := clustersService.List().Send()
	if err != nil {
		return err
	}
	clusters, ok := clustersResponse.Clusters()
	if !ok {
		return fmt.Errorf("Could not get cluster list or it is empty")
	}
	c.clusters = clusters

	return err
}

func (c *OVirtCollector) getAllDatacentersHosts(ctx context.Context) error {
	var err error

	if time.Since(c.lastHoUpdate) < c.dataDuration {
		return nil
	}
	if err = c.getDatacentersAndClusters(ctx); err != nil {
		return err
	}

	// Get hosts
	hostsService := c.conn.SystemService().HostsService()
	hostsResponse, err := hostsService.List().Send()
	if err != nil {
		return err
	}
	hosts, ok := hostsResponse.Hosts()
	if !ok {
		return fmt.Errorf("Could not get hosts list or it is empty")
	}
	c.hosts = hosts
	c.lastHoUpdate = time.Now()

	return err
}

func (c *OVirtCollector) getAllDatacentersStorageDomains(ctx context.Context) error {
	var err error

	if time.Since(c.lastSdUpdate) < c.dataDuration {
		return nil
	}
	if err = c.getDatacentersAndClusters(ctx); err != nil {
		return err
	}

	// Get storage domains
	sdsService := c.conn.SystemService().StorageDomainsService()
	sdsResponse, err := sdsService.List().Send()
	if err != nil {
		return err
	}
	sds, ok := sdsResponse.StorageDomains()
	if !ok {
		return fmt.Errorf("Could not get storagedomain list or it is empty")
	}
	c.sds = sds
	c.lastSdUpdate = time.Now()

	return nil
}

func (c *OVirtCollector) getAllDatacentersVMs(ctx context.Context) error {
	var err error

	if time.Since(c.lastVmUpdate) < c.dataDuration {
		return nil
	}
	if err = c.getAllDatacentersHosts(ctx); err != nil {
		return err
	}

	// Get all VMs
	vmsService := c.conn.SystemService().VmsService()
	vmsResponse, err := vmsService.List().Send()
	if err != nil {
		return err
	}
	vms, ok := vmsResponse.Vms()
	if !ok {
		return fmt.Errorf("Could not get VM list or it is empty")
	}
	c.vms = vms
	c.lastVmUpdate = time.Now()

	return nil
}

// datacenterNameFromId returns the datacenter name given its Id
func (c *OVirtCollector) datacenterNameFromId(id string) string {
	var clid, name string
	var ok bool

	for _, cl := range c.dcs.Slice() {
		if clid, ok = cl.Id(); ok {
			if clid == id {
				name, _ = cl.Name()
				break
			}
		}
	}
	return name
}

// countClustersInDc returns the number of cluster in the given DC by its Id
func (c *OVirtCollector) countClustersInDc(id string) int16 {
	var (
		edc       *ovirtsdk.DataCenter
		dcid      string
		nclusters int16 = 0
		ok        bool
	)

	for _, cl := range c.clusters.Slice() {
		if edc, ok = cl.DataCenter(); ok {
			if dcid, ok = edc.Id(); ok {
				if dcid == id {
					nclusters++
				}
			}
		}
	}
	return nclusters
}

// clusterName returns a cluster's name from cache
func (c *OVirtCollector) clusterName(cl *ovirtsdk.Cluster) string {
	var clid, id, name string
	var ok bool

	if id, ok = cl.Id(); !ok {
		return name
	}
	for _, cl := range c.clusters.Slice() {
		if clid, ok = cl.Id(); ok {
			if clid == id {
				name, _ = cl.Name()
				break
			}
		}
	}
	return name
}

// clusterDatacenterName returns a cluster's datacenter name from cache
func (c *OVirtCollector) clusterDatacenterName(cl *ovirtsdk.Cluster) string {
	var dc *ovirtsdk.DataCenter
	var clid, id, dcid, name string
	var ok bool

	if id, ok = cl.Id(); !ok {
		return name
	}
	for _, cl := range c.clusters.Slice() {
		if clid, ok = cl.Id(); ok {
			if clid == id {
				if dc, ok = cl.DataCenter(); ok {
					if dcid, ok = dc.Id(); ok {
						name = c.datacenterNameFromId(dcid)
						break
					}
				}
			}
		}
	}
	return name
}

// hostName returns a host's name from cache
func (c *OVirtCollector) hostName(ho *ovirtsdk.Host) string {
	var hoid, id, name string
	var ok bool

	if id, ok = ho.Id(); !ok {
		return name
	}
	for _, h := range c.hosts.Slice() {
		if hoid, ok = h.Id(); ok {
			if hoid == id {
				name, _ = h.Name()
				break
			}
		}
	}
	return name
}
