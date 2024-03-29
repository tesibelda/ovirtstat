# Metrics

- ovirtstat_apisummary
  - tags:
    - ovirt-engine
  - fields:
	- hosts (int)
	- storagedomains (int)
	- users (int)
    - version (string)
	- vms_active (int)
	- vms_total (int)
- ovirtstat_datacenter
  - tags:
    - name
    - ovirt-engine
  - fields:
    - clusters (int)
	- local (bool)
	- status (string)
	- status_code (int) 0-up, 1-maintenance, 2-uninitialized, 3-problematic, 4-contend, 5-notoperational
- ovirtstat_host
  - tags:
    - clustername
    - dcname
    - name
	- id
    - ovirt-engine
	- type
  - fields:
    - cpu_cores (int)
    - cpu_sockets (int)
    - cpu_speed (float)
    - cpu_threads (int)
	- memory_size (int) in bytes
	- reinstallation_required (bool)
	- status (string)
	- status_code (int) 0-up, 1-maintenance, 2..8-misc, 9-error, 10-nonresponsive, 11-nonoperational, 12-down
	- vm_active (int)
	- vm_migrating (int)
	- vm_total (int)
- ovirtstat_storagedomain
  - tags:
	- id
    - name
    - ovirt-engine
	- storage_type
    - type
  - fields:
	- available (int) in bytes
	- committed (int) in bytes
	- connections (int)
	- external_status (string)
	- external_status_code (int) 0-ok, 1-info, 2-warning, 3-error, 4-failure
	- logical_units (int)
	- master (bool)
	- used (int) in bytes
	- status (string)
	- status_code (int) 0-active, 1-activating, 2-maintenance, 3-unknown, 4-detaching, 5-unattached, 6-mixed, 7-locked
- ovirtstat_glustervolume
  - tags:
    - clustername
    - dcname
	- id
    - name
    - ovirt-engine
	- type
  - fields:
	- briks (int)
	- disperse_count (int)
	- redundancy_count (int)
	- replica_count (int)
	- status (string)
	- status_code (int) 0-up, 1-unknown, 2-down
	- stripe_count (int)
- ovirtstat_vm
  - tags:
    - clustername
    - dcname
    - hostname
	- id
    - name
    - ovirt-engine
	- type
  - fields:
    - cpu_cores (int)
    - cpu_sockets (int)
    - cpu_threads (int)
	- memory_size (int) in bytes
	- run_once (bool)
	- stateless (bool)
	- status (string)
	- status_code (int) 0-up, 1-paused, 2..9-misc, 10-unknown, 11-unassigned, 12-notresponding, 13-down
- internal_ovirtstat
  - tags:
    - alias
    - ovirt-engine
    - ovirtstat_version
  - fields:
    - sessions_created (int)
    - gather_time_ns (int)