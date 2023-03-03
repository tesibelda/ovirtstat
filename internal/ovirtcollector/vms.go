// This file contains ovirtcollector methods to gathers stats about VMs
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package ovirtcollector

import (
	"context"
	"fmt"

	"github.com/influxdata/telegraf"
	//ovirtsdk "github.com/ovirt/go-ovirt"
)

// CollectVmsInfo gathers oVirt VMs info
func (c *OVirtCollector) CollectVmsInfo(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var err error

	if c.conn == nil {
		return fmt.Errorf("Could not get VMs info: %w", ErrorNoClient)
	}

	if err = c.getAllDatacentersVMs(ctx); err != nil {
		return fmt.Errorf("Could not get all VM entity lists: %w", err)
	}

	// TODO

	return nil
}
