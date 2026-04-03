# xman Guides

This directory is the task-oriented documentation set for `xman`.

The main `README.md` explains what the project is and which features it has.
These guides explain how to use those features to get real work done.

## Feature Coverage

The guide set is intended to cover the full user-facing surface of the app:

- backend connection and backend choice
- VM browsing and selection
- power operations
- VM config edits
- VM NIC attachment edits
- VMware Tools bootstrap and repair workflows
- browser console launch
- snapshots
- command execution through Guest Ops and SSH
- file transfer through Guest Ops and SFTP
- SSH key deployment
- SSH key management
- guest credential management
- stored script management
- global networks view
- host and datastore inventory
- job progress, logs, cancellation, and dismissal
- offline and air-gapped operator recipes

## How to Use These Guides

Most guides follow the same structure:

- Goal: what you are trying to accomplish
- Requirements: what must already be in place
- Recommended transport: Guest Ops or SSH, and why
- Steps in xman: the UI flow to use
- Example commands: guest-side commands to run
- Verification: how to confirm the action worked
- Troubleshooting: common reasons the workflow can fail

When a guide says "guest", it means the VM you are managing through `xman`.
When it says "host", it means the machine running `xman`.

## Guide Index

- [Connecting to Backends](./connecting.md)
- [VM Management](./vm-management.md)
- [Browser Console](./console.md)
- [Snapshots](./snapshots.md)
- [Running Commands in a Guest](./run-commands.md)
- [Transferring Files](./file-transfer.md)
- [Credentials, SSH Keys, and Key Deployment](./credentials-and-keys.md)
- [Stored Scripts](./scripts.md)
- [Networks and Inventory](./networks-and-inventory.md)
- [Jobs and Terminal Output](./jobs-and-terminal.md)
- [Backend Differences](./backend-differences.md)
- [Install Python Offline](./install-python-offline.md)
- [Install Pandas Offline](./install-pandas-offline.md)
- [Troubleshooting](./troubleshooting.md)

## Common Behavior Notes

These are the main sources of "that was unexpected" feedback:

- Guest Ops readiness can lag behind VM power-on, even when the VM looks up.
- The `Run` tab is not a live shell. Each run is a separate session.
- Starting a new in-app run replaces the previous in-app output pane.
- Stored `.ps1` scripts can be saved, but are not supported in the `Run` tab.
- File transfer is currently file-oriented, not directory-oriented.
- The browser console depends on desktop reachability to vCenter and the console
  host, not to the guest IP.
- `Inventory` is a `vCenter` feature. It is not shown for `Workstation`.
- Some edit operations are intentionally power-state dependent, especially on
  `Workstation`.
- Job cancellation is most meaningful for command execution jobs.

## Writing Style for Future Guides

Prefer guides that are concrete and operator-focused.

Good guide titles:

- "Install Python Offline on a Windows Guest"
- "Upload a Script Bundle and Run It with Guest Ops"
- "Bootstrap SSH Access with a Password and Then Switch to Keys"

Less useful guide titles:

- "File Transfer Feature"
- "Guest Operations Overview"

If a workflow depends on backend behavior, call that out early instead of
burying it in troubleshooting notes.
