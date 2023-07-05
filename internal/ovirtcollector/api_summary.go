// This file contains ovirtcollector methods to gathers stats about oVirt API
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package ovirtcollector

import (
	"context"
	"fmt"
	"time"

	ovirtsdk "github.com/ovirt/go-ovirt"
	"github.com/tesibelda/lightmetric/metric"
)

// CollectAPISummaryInfo gathers oVirt api's summary info
func (c *OVirtCollector) CollectAPISummaryInfo(
	ctx context.Context,
	acc *metric.Accumulator,
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
		return fmt.Errorf("could not get oVirt API info: %w", ErrorNoClient)
	}

	if apiSvc, err = c.conn.SystemService().Get().Send(); err != nil {
		return err
	}
	if api, ok = apiSvc.Api(); !ok {
		return fmt.Errorf("could not get oVirt API data")
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
