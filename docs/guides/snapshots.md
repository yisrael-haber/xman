# Snapshots

## Goal

Use `xman` to list, create, revert, and delete VM snapshots.

## Main Actions

In the VM `Snapshots` tab you can:

- refresh the snapshot list
- create a new snapshot
- revert to an existing snapshot
- delete a snapshot

## Backend Differences

On both backends:

- snapshot name is supported
- revert is supported
- delete is supported

On `vCenter` only:

- description is supported
- memory capture is supported
- quiesce is supported

## Recommended Usage

Use snapshots before:

- risky software installs
- guest configuration experiments
- one-off troubleshooting changes

## Unexpected Behavior Sources

- `Workstation` supports a simpler snapshot model than `vCenter`.
- Quiesce depends on VMware Tools and guest support.
- The current UI deletes snapshots without exposing a "remove children" option.
- Snapshot tree depth and "current" markers can be richer on `vCenter` than on
  `Workstation`.
