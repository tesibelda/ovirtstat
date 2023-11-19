// ovirtcollector package allows you to gather basic stats from oVirt Engine
//
//  Use New method to create a new struct, Open to open a session with a oVirt and then
// use Collect* methods to get metrics added to a telegraf accumulator and finally
// Close when finished.
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package ovirtcollector

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/influxdata/telegraf/filter"
	"github.com/influxdata/telegraf/plugins/common/tls"
	ovirtsdk "github.com/ovirt/go-ovirt"
	"github.com/tesibelda/ovirtstat/internal/netplus"
)

// Common raised errors
var (
	ErrorNoClient = errors.New("no oVirt connection has been opened")
	ErrorNotVC    = errors.New("endpoint does not look like an oVirt Engine")
	ErrorURLNil   = errors.New("oVirt Engine URL should not be nil")
)

// OVirtCollector struct contains session and entities of a OVirt
type OVirtCollector struct {
	tls.ClientConfig
	urlString, user, pass string
	url                   *url.URL
	conn                  *ovirtsdk.Connection
	filterClusters        filter.Filter
	filterHosts           filter.Filter
	filterVms             filter.Filter
	dataDuration          time.Duration
	VcCache
}

// New returns a new OVirtCollector associated with the provided OVirt URL
func New(
	ovirtURL, user, pass string,
	clicfg *tls.ClientConfig,
	dataDuration time.Duration,
) (*OVirtCollector, error) {
	var err error

	ovc := OVirtCollector{
		urlString:    ovirtURL,
		user:         user,
		pass:         pass,
		conn:         nil,
		dataDuration: dataDuration,
	}
	if err = ovc.SetFilterClusters(nil, nil); err != nil {
		return nil, err
	}
	if err = ovc.SetFilterHosts(nil, nil); err != nil {
		return nil, err
	}
	if err = ovc.SetFilterVms(nil, nil); err != nil {
		return nil, err
	}
	ovc.TLSCA = clicfg.TLSCA
	ovc.InsecureSkipVerify = clicfg.InsecureSkipVerify

	ovc.url, err = netplus.PaseURL(ovirtURL, user, pass)
	if err != nil {
		return nil, err
	}

	return &ovc, err
}

// SetDataDuration sets max cache data duration
func (c *OVirtCollector) SetDataDuration(du time.Duration) {
	c.dataDuration = du
}

// SetFilterClusters sets clusters include and exclude filters
func (c *OVirtCollector) SetFilterClusters(include, exclude []string) error {
	var err error

	c.filterClusters, err = filter.NewIncludeExcludeFilter(include, exclude)
	if err != nil {
		return err
	}
	return nil
}

// SetFilterHosts sets hosts include and exclude filters
func (c *OVirtCollector) SetFilterHosts(include, exclude []string) error {
	var err error

	c.filterHosts, err = filter.NewIncludeExcludeFilter(include, exclude)
	if err != nil {
		return err
	}
	return nil
}

// SetFilterVms sets VMs include and exclude filters
func (c *OVirtCollector) SetFilterVms(include, exclude []string) error {
	var err error

	c.filterVms, err = filter.NewIncludeExcludeFilter(include, exclude)
	if err != nil {
		return err
	}
	return nil
}

// Open opens a OVirt connection session
func (c *OVirtCollector) Open(_ context.Context, timeout time.Duration) error {
	var err error

	c.conn, err = ovirtsdk.NewConnectionBuilder().
		URL(c.urlString).
		Username(c.user).
		Password(c.pass).
		Insecure(c.InsecureSkipVerify).
		Compress(true).
		Timeout(timeout).
		Build()

	return err
}

// IsActive let us know if the OVirt connection is active or not
func (c *OVirtCollector) IsActive(_ context.Context) bool {
	if c.conn != nil && c.conn.Test() == nil {
		return true
	}
	return false
}

// Close closes OVirt connection
func (c *OVirtCollector) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}
