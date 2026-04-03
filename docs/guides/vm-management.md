# VM Management

## Goal

Handle the core VM actions from `xman`:

- review VM details
- power the VM on or off
- reset or suspend it
- edit basic VM configuration
- reattach virtual NICs
- trigger VMware Tools installation or repair flows

## Main Actions

In the VM `Info` tab you can:

- review guest OS, IP, hostname, firmware, UUID, and placement details
- power on, power off, reset, or suspend the VM
- edit name, notes, CPU, memory, and firmware when allowed
- change individual VM NIC attachment and connected-on-boot state
- trigger VMware Tools workflows when the backend supports them

## Power Actions

Use:

- `Power On` to boot a stopped VM
- `Power Off` for a hard stop
- `Reset` for a guest reboot equivalent to power cycling
- `Suspend` when you want to preserve memory state

## Config Edits

Typical config edits:

- VM name
- notes
- vCPU count
- memory
- firmware

## Network Attachment Edits

Use the NIC editor to:

- pick a different available network
- change whether the NIC is connected at power on

## VMware Tools Workflows

The VM info view can help with:

- repairing VMware Tools
- upgrading VMware Tools
- bootstrapping Guest Ops on supported Windows workflows

For Linux and macOS guests, package-based `open-vm-tools` installation is still
the normal path.

## Unexpected Behavior Sources

- On `Workstation`, configuration is VMX-backed, so most edits unlock only when
  the VM is powered off.
- On `vCenter`, name and notes can be more flexible than hardware settings while
  the VM is running.
- NIC reattachment currently requires the VM to be powered off.
- The Tools action is most meaningful for Windows guests. Linux and macOS
  workflows usually stay package-manager driven.
