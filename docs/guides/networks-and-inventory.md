# Networks and Inventory

## Goal

Use the global infrastructure views to understand the environment around your
VMs.

## Networks

The `Networks` screen is available on both backends.

On `vCenter`, it helps you inspect:

- standard switches
- distributed switches
- port groups
- VLAN details
- attached hosts and VMs

On `Workstation`, it helps you inspect:

- VMnet interfaces
- MTU
- host IP configuration
- connected VMs

## Inventory

The `Hosts & Datastores` screen is available on `vCenter` only.

It provides:

- ESXi host inventory
- CPU and memory usage
- datastore capacity and free space

## Unexpected Behavior Sources

- `Networks` exists on both backends, but the meaning of the data differs by
  backend.
- `Inventory` is intentionally absent on `Workstation`.
- VM NIC reattachment is handled from the VM detail screen, not from the global
  network inventory screen.
