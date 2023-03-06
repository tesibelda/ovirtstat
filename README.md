# ovirtstat execd input for telegraf

ovirtstat is an [oVirt](https://www.ovirt.org/) input plugin for [Telegraf](https://github.com/influxdata/telegraf) that gathers status and basic stats from oVirt Engine using [go-ovirt](https://github.com/ovirt/go-ovirt)

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://github.com/tesibelda/ovirtstat/raw/master/LICENSE)

# Compatibility

Latest releases are built with a go-ovirt library version that should work with oVirt 4.x. 

# Configuration

* Download the [latest release package](https://github.com/tesibelda/ovirtstat/releases/latest) for your platform.

* Edit ovirtstat.conf file as needed. Example:

```toml
[[inputs.ovirtstat]]
  ## OVirt Engine URL to be monitored and its credential
  ovirturl = "https://ovirt-engine.local/ovirt-engine/api"
  username = "user@internal"
  password = "secret"

  ## Optional SSL Config
  # tls_ca = "/path/to/cafile"
  ## Use SSL but skip chain & host verification
  # insecure_skip_verify = false

  ## optional alias tag for internal metrics
  # internal_alias = ""

  ## Filter clusters by name, default is no filtering
  ## cluster names can be specified as glob patterns
  # clusters_include = []
  # clusters_exclude = []

  ## Filter hosts by name, default is no filtering
  ## host names can be specified as glob patterns
  # hosts_include = []
  # hosts_exclude = []

  ## Filter VMs by name, default is no filtering
  ## VM names can be specified as glob patterns
  # vms_include = []
  # vms_exclude = []

  ## Filter collectors by name, default is all collectors
  ## see possible collector names bellow
  # collectors_include = []
  # collectors_exclude = []

  #### collector names available are ####
  ## Datacenters: datacenter stats in ovirtstat_datacenter measurement
  ## Hosts: hypervisor/host stats in ovirtstat_host measurement
  ## StorageDomains: cluster stats in ovirtstat_storagedomains measurement
  ## VMs: virtual machine stats in ovirtstat_vm measurement
```

* Edit telegraf's execd input configuration as needed. Example:

```
## Gather vSphere vCenter status and basic stats
[[inputs.execd]]
  command = ["/path/to/ovirtstat_binary", "-config", "/path/to/ovirtstat.conf"]
  signal = "none"
```

You can optionally tell ovirtstat the input's interval by adding -poll_interval the_interval parameters to the command. By default it expects 1m interval. If you want 30s interval configure it like this:
```
## Gather vSphere vCenter status and basic stats
[[inputs.execd]]
  interval = "30s"
  command = ["/path/to/ovirtstat_binary", "-config", "/path/to/ovirtstat.conf", "-poll_interval", "30s"]
  signal = "none"
```
Metric timestamp precision will be set according to the polling interval, so it will usually be 1s.

* Restart or reload Telegraf.

# Quick test in your environment

* Edit ovirtstat.conf file as needed (see above)

* Run ovirtstat with --config argument using that file.
```
/path/to/ovirtstat --config /path/to/ovirtstat.conf
```

* Wait for 1 minute or press enter. You should see lines like those in the Example output below.


# Example output

```plain
ovirtstat_apisummary,ovirt-engine=myovirt users=5i,vms=8i,version="4.4.10.7-1.0.17.el8",hosts=5i,storagedomains=23i 1677832223000000000
ovirtstat_datacenter,name=mydc,id=c3a7efc0-8417-4d1b-bc74-fa6f20d6bf1f,ovirt-engine=myovirt status="up",status_code=0i,clusters=1i 1677832223000000000
ovirtstat_host,clustername=mycluster,dcname=mydc,id=b3d53f5d-7ec3-43a8-a52a-15fe7dde25c2,name=myhyp01,ovirt-engine=myovirt cpu_threads=2i,memory_size=1622535045120i,status="up",status_code=0i,cpu_cores=16i,cpu_sockets=2i,cpu_speed=800 1677832224000000000
ovirtstat_storagedomain,id=072cba31-08f3-4a40-9f24-a5ca22ed1d74,name=ovirt-image-repository,ovirt-engine=myovirt,type=image available=0i,connections=0,commited=0i,external_status="ok",external_status_code=0,master=false,status="unattached",status_code=5i,used=0i 1677832224000000000
ovirtstat_storagedomain,id=ec413fb2-c6ce-4bea-a790-2533b728ac93,name=mysd01,ovirt-engine=myovirt,type=data available=3233036632064i,connections=7,commited=16603269824512i,external_status="ok",external_status_code=0,master=true,status="",status_code=3i,used=7761005903872i 1677832224000000000
internal_ovirtstat,ovirt-engine=myovirt sessions_created=1i,gather_time_ns=803780400i 1677832224000000000
```

# Metrics
See [Metrics](https://github.com/tesibelda/ovirtstat/blob/master/METRICS.md)

# Build Instructions

Download the repo

    $ git clone git@github.com:tesibelda/ovirtstat.git

build the "ovirtstat" binary

    $ go build -o bin/ovirtstat cmd/main.go
    
 (if you're using windows, you'll want to give it an .exe extension)
 
    $ go build -o bin\ovirtstat.exe cmd/main.go

 If you use [go-task](https://github.com/go-task/task) execute one of these
 
    $ task linux:build
	$ task windows:build

# Author

Tesifonte Belda (https://github.com/tesibelda)

# License

[The MIT License (MIT)](https://github.com/tesibelda/ovirtstat/blob/master/LICENSE)
