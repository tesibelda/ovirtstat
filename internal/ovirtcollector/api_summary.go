// This file contains ovirtcollector methods to gathers stats about oVirt API
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

// CollectApiSummaryInfo gathers oVirt api's summary info
func (c *OVirtCollector) CollectApiSummaryInfo(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		apiSvc    *ovirtsdk.SystemServiceGetResponse
		api       *ovirtsdk.Api
		vms       *ovirtsdk.ApiSummaryItem
		apitags   = make(map[string]string)
		apifields = make(map[string]interface{})
		t         time.Time
		err       error
		ok        bool
	)

	if c.conn == nil {
		return fmt.Errorf("Could not get oVirt API info: %w", ErrorNoClient)
	}

	if apiSvc, err = c.conn.SystemService().Get().Send(); err != nil {
		return err
	}
	if api, ok = apiSvc.Api(); !ok {
		return fmt.Errorf("Could not get oVirt API data")
	}
	t = time.Now()

	apitags["ovirt-engine"] = c.url.Host

	apifields["version"] = api.MustProductInfo().MustVersion().MustFullVersion()
	apifields["hosts"] = api.MustSummary().MustHosts().MustTotal()
	apifields["storagedomains"] = api.MustSummary().MustStorageDomains().MustTotal()
	apifields["users"] = api.MustSummary().MustUsers().MustTotal()
	if vms, ok = api.MustSummary().Vms(); ok {
		apifields["vms_active"], _ = vms.Active()
		apifields["vms_total"], _ = vms.Total()
	}

	acc.AddFields("ovirtstat_apisummary", apifields, apitags, t)

	return nil
}
