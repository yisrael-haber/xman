# Connecting to Backends

## Goal

Connect `xman` to either `vCenter` or `VMware Workstation` and understand which
backend to use for a given environment.

## Requirements

- `xman` is running
- You know whether your target VM is managed by `vCenter` or by local
  `VMware Workstation`

## Choose a Backend

Use `vCenter` when:

- your VM is managed by ESXi/vCenter
- you want inventory and datastore views
- you want the browser console flow

Use `Workstation` when:

- your VM is managed locally through `vmrun`
- you are browsing a local Workstation library or VM directory
- you do not need vCenter-only inventory features

## Connect to vCenter

1. Open the login screen.
2. Select `vCenter`.
3. Enter the vCenter URL, username, and password.
4. Decide whether to allow self-signed certificates.
5. Click `Connect`.

## Connect to Workstation

1. Open the login screen.
2. Select `Workstation`.
3. Optionally choose a VM folder.
4. Leave the VM folder empty to use the default Workstation inventory.
5. Click `Connect`.

## Verification

After connecting, confirm:

- the header shows the connected backend name
- the VM list loads
- the VM details pane updates when you select a VM

## Notes

- `Workstation` depends on `vmrun` being installed and reachable.
- `vCenter` depends on guest permissions when you later use Guest Ops.
- Some features appear only when the connected backend supports them.
