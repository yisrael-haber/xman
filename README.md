# xman

Unified desktop console for VMware VM lifecycle, guest operations, and SSH workflows.

`xman` is a native desktop application for day-to-day VMware management. It is built with Go and Wails and currently supports both **vCenter** and **VMware Workstation** backends.

The app is meant to make common operator workflows fast:
- browse VMs
- power-cycle them
- run commands and stored scripts
- transfer files
- deploy SSH keys
- manage reusable scripts
- adjust VM NIC attachments
- inspect inventory and networks

It is designed for operators who routinely bounce between hypervisor actions and in-guest actions and want those workflows in one place instead of split across the vSphere Client, Workstation UI, SSH terminals, and ad hoc scripts.

## Project Status

`xman` is currently best described as a serious internal operator tool rather than a fully hardened enterprise product.

Today it is strongest for:
- individual operators
- homelabs and lab environments
- small infrastructure/platform teams
- controlled internal use on known environments

The core workflows are in good shape, the codebase has meaningful automated coverage, and the app is already useful in practice. At the same time, environment-specific validation, security hardening, and broader distribution polish are still ongoing.

## Highlights

### VM Management
- Browse and select VMs with power state, VMware Tools state, IP address, CPU, and memory
- Navigate a hierarchy-aware VM tree:
  - `vCenter` uses inventory folders / vApps
  - `Workstation` uses library folders from `inventory.vmls` when available, with filesystem hierarchy as fallback
- Filter the VM browser by VM name, folder path, guest OS, IP, or VM reference
- Launch a browser console for `vCenter` VMs using the VMware HTML5 console page served by vCenter
- Power on, power off, reset, and suspend VMs
- Edit per-adapter network attachment and connected-on-boot state while the VM is powered off
- Keep job completion and VM state refresh synchronized

### Snapshots
- List snapshots with creation time and tree depth when the backend provides them
- Create snapshots on both backends
- On vCenter, snapshots also support description, memory capture, and quiesce options
- Revert to or delete snapshots

### Run
Run guest commands inside a VM using two transports:

- **Guest Ops**: uses a selected stored guest credential and requires VMware Tools
- **SSH**: uses `host + SSH key` only; the SSH username comes from the selected key's default user

The Run tab is intentionally modeled as **separate shell sessions**, not a live terminal. Each new run replaces the previous output in the output pane.

It supports two execution modes:
- **Raw Command**: enter the command directly
- **Stored Script**: choose a saved script from the global **Scripts** feature

It also supports:
- copy output to clipboard
- cancel a running command without restarting the app
- launch a real interactive SSH shell in the native OS terminal
- background job tracking with detailed logs

The interactive SSH launcher uses:
- the selected key's private key file
- the key's default SSH user
- the native `ssh` client and terminal on the host OS

It is intentionally separate from the in-app command console.

### Scripts
- Manage reusable stored scripts from the global **Scripts** feature
- Scripts are stored as regular files under the app config directory in `xman/scripts`
- Run stored scripts from the VM **Run** tab using the same transport and job model as raw commands
- Script type is inferred from the filename:
  - `.sh` for POSIX guests
  - `.cmd` / `.bat` for Windows guests
  - `.txt` for generic text snippets
- `.ps1` files can be stored, but PowerShell-specific execution is intentionally not wired up yet

### File Transfer
Upload and download files to or from a guest VM using:

- **Guest Ops**
- **SSH / SFTP**

For SSH / SFTP, the app uses a stored private key plus that key's default user. For Guest Ops transfers, the app uses a selected stored guest credential. Guest passwords are not used for day-to-day SSH transfers.

### SSH Keys & Guest Credentials
- Generate SSH key pairs inside the app
- Store key metadata including a default SSH username
- Store labeled guest credentials for VMware Guest Ops
- View, edit, and delete stored guest credentials without retyping them for every run
- Deploy the public key to a guest using a one-time password bootstrap flow
- Use deployed keys for SSH / SFTP afterward without re-entering passwords
- Use saved guest credentials from a dropdown in Run and File Transfer

### Deploy SSH Key
From the VM panel, deploy a selected SSH public key to a guest using:
- host
- port
- remote username
- remote password

This is the only SSH flow that still uses password authentication by design.

### Networks
- **vCenter**: inspect standard switches, distributed switches, port groups, VLAN information, uplinks, attached hosts, and connected VMs
- **Workstation**: inspect VMnet interfaces on the host OS, including MTU, IP configuration, and connected VMs
- Reattach individual VM NICs to a different available network and change whether they connect at power on

### Inventory
Available on **vCenter**:
- ESXi host inventory with CPU and memory utilization
- datastore inventory with capacity and free space

### VMware Tools Management
- Install or upgrade VMware Tools on Windows guests
- On vCenter: trigger built-in Tools upgrade when available, otherwise mount the Tools ISO
- On Workstation: mount the bundled Tools ISO and start installation
- For Linux and macOS guests, use `open-vm-tools` from the guest OS package manager

### Browser Console
- Available on `vCenter`
- Opens the VMware HTML5 console in the default browser with a fresh one-time session ticket
- Shows console diagnostics in the VM tab before launch, including the selected console host, redacted launch URL, and simple reachability checks
- Prefers the exact vCenter host you connected to when building the console URL, which helps avoid closed-network FQDN mismatches
- Does not require public internet access because the console page is served by vCenter itself
- Requires desktop reachability to:
  - the `vCenter` endpoint
  - the ESXi host currently serving the target VM's console
- Does not require direct network reachability to the guest VM IP

### Job Tracking
Long-running operations are tracked as jobs with:
- progress
- detailed log history
- completion duration
- target VM labeling
- dismiss for completed jobs
- cancel for running command jobs

## Backend Comparison

| Feature                  | vCenter | Workstation |
|--------------------------|:-------:|:-----------:|
| VM listing               | ✓       | ✓           |
| Browser console          | ✓       | —           |
| Power operations         | ✓       | ✓           |
| Snapshots                | ✓       | ✓           |
| Run (Guest Ops)          | ✓       | ✓           |
| Run (SSH)                | ✓       | ✓           |
| File Transfer (Guest Ops)| ✓       | ✓           |
| File Transfer (SSH/SFTP) | ✓       | ✓           |
| Deploy SSH Key           | ✓       | ✓           |
| Networks view            | ✓       | ✓           |
| Host inventory           | ✓       | —           |
| Datastore inventory      | ✓       | —           |
| Snapshot description / quiesce / memory options | ✓ | — |
| Tools auto-upgrade       | ✓ (Windows) | —       |
| Tools ISO install        | ✓ (fallback, Windows) | ✓ (Windows) |

## How SSH Works

The app now uses a lightweight SSH model:

1. Create an SSH key pair in the **SSH Keys** panel
2. Set a default user on that key
3. Use **Deploy SSH Key** once to push the public key to the guest with a password
4. After that, Run / File Transfer over SSH use:
   - `host`
   - `key`
   - the key's default user

For a true live shell, the Run tab can also launch the native OS terminal with the system `ssh` client and the selected private key.

This avoids keeping per-VM deployment state in local config and keeps the runtime behavior predictable:
- if the selected key works for `user@host`, SSH succeeds
- if it does not, SSH fails cleanly

## Guest Ops Requirements

Guest Ops transport for Run and File Transfer requires:
- VMware Tools or open-vm-tools installed and running
- a stored guest credential
- on vCenter, guest operations privileges on the VM

Linux guests can usually use `open-vm-tools`:

```bash
# Debian / Ubuntu
sudo apt install open-vm-tools

# RHEL / CentOS / Rocky
sudo yum install open-vm-tools
```

If VMware Tools are not available, switch to the SSH-based transport instead.

The UI will still let you attempt Guest Ops on a powered-on VM even before the Tools state has been positively determined. If Tools are missing or not running, the backend error is surfaced in the tab instead of being blocked up front.

## Prerequisites

### Go

Go 1.21 or later:

```bash
go version
```

### Node.js

Node 18 or later:

```bash
node --version
npm --version
```

### System Libraries (Linux / WSL2)

On Ubuntu 24.04:

```bash
sudo apt update && sudo apt install -y \
  build-essential \
  pkg-config \
  libgtk-3-dev \
  libwebkit2gtk-4.1-dev \
  libsecret-1-dev \
  gnome-keyring
```

On Ubuntu 22.04 or older, replace `libwebkit2gtk-4.1-dev` with `libwebkit2gtk-4.0-dev`.

### Wails CLI

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails doctor
```

### WSL2 Note

Wails renders a native GUI window. On WSL2, that requires **WSLg**:

```bash
echo $DISPLAY
```

If `$DISPLAY` is empty, update WSL from Windows:

```powershell
wsl --update
```

## Getting Started

```bash
git clone <repo-url>
cd xman

cd frontend && npm install && cd ..

# Development
wails dev -tags webkit2_41

# Linux build
wails build -tags webkit2_41

# Windows build from Linux / WSL2
wails build -platform windows/amd64
```

For Ubuntu 24.04, `-tags webkit2_41` is required because the distro ships `webkit2gtk-4.1`.

## Useful Make Targets

```bash
make help
make dev
make build
make build-windows
make test
make test-go
make test-go-cached
make test-vcenter
make test-vcenter-docker
make test-workstation
make test-workstation-integration
make test-frontend
make test-all
make test-all-cached
make vet
```

There is also a `vcsim` helper target for local vCenter-style simulator work:

```bash
make vcsim
```

By default this starts a foldered simulator model plus an additional richer demo tree for browsing the VM hierarchy in the UI. Useful overrides:

```bash
make vcsim DEMO_TREE=false
make vcsim DC=2 FOLDER=2 VM=3
```

## Workstation Performance Logging

The Workstation backend includes a lightweight performance logging mode for investigating refresh latency and expensive `vmrun` calls.

Run the built application with:

```powershell
.\xman.exe perflog
```

That creates a timestamped log file like:

```text
xman_log_YYYYMMDD_HHMMSS.txt
```

This is mainly intended for debugging Workstation refresh behavior and can be expanded further later.

## Testing

The current test strategy is intentionally split into fast local tests and opt-in environment-backed tests.

Recommended commands:

```bash
# Default uncached Go test run
make test

# Full validation pass: uncached Go tests + frontend production build
make test-all

# Faster reruns when you explicitly want Go's test cache
make test-go-cached
make test-all-cached

# Focused backend runs
make test-vcenter
make test-vcenter-docker
make test-workstation
```

What these do:

- `make test`
  - runs all Go tests with `-count=1`
  - includes manager tests
  - includes `vCenter` tests backed by `vcsim`
  - includes Workstation tests backed by pure helpers and a fake `vmrun`
- `make test-all`
  - runs the same uncached Go suite
  - also runs `cd frontend && npm run build`
- `make test-go-cached` / `make test-all-cached`
  - same as above, but allow Go to return cached package test results
- `make test-vcenter`
  - runs only the `vCenter` backend tests
- `make test-vcenter-docker`
  - runs only the Docker-backed `vCenter` guest-ops integration tests
  - covers stable guest-ops integration paths against container-backed `vcsim` VMs
  - includes hard assertions for guest command success and guest file upload/download
  - includes probe-style tests for cancellation and non-zero exit behavior, which intentionally skip with observations when `vcsim` does not surface those semantics reliably
- `make test-workstation`
  - runs only the Workstation backend tests

Important note:

- use `make test-all`, not `make test all`

### Workstation Integration Tests

There is also an opt-in smoke test target for a real VMware Workstation setup reachable from WSL:

```bash
XMAN_WS_VMRUN='/mnt/c/Program Files (x86)/VMware/VMware Workstation/vmrun.exe' \
XMAN_WS_VM_DIR='/mnt/c/path/to/test/vms' \
make test-workstation-integration
```

This target is intentionally not part of the normal suite.

It is meant for:
- validating real `vmrun` invocation from WSL
- listing real VMs from a dedicated test directory
- smoke-testing the Workstation environment without requiring it for every run

Notes:
- no `sudo` should be required for the normal test targets
- the `vCenter` tests use a localhost `vcsim` server internally
- the Workstation integration target should point at disposable or non-critical test VMs

### Docker-backed vCenter Guest Ops Tests

There is also an opt-in Docker-backed simulator target for `vCenter` guest-ops coverage:

```bash
make test-vcenter-docker
```

This target is intentionally separate from `make test` and `make test-all` so the default workflow stays fast and does not depend on Docker.

It currently covers:
- guest command success
- guest file upload/download round-trips
- explicit probe attempts for cancellation and non-zero exit behavior

Notes:
- the Docker-backed tests found and now protect a small guest-output download retry in the `vCenter` backend, which helps when process completion races slightly ahead of guest file visibility
- cancellation and non-zero-exit probe cases are intentionally best-effort in this layer; they record what the simulator exposes instead of forcing simulator-specific behavior into the production backend
- non-zero guest command exit semantics are still covered by the shared manager/unit tests rather than the Docker-backed `vcsim` layer, because the container guest-process simulator does not currently report those exit codes reliably

Requirements:
- Linux or WSL2
- Docker available inside the distro (`docker info` must succeed)
- ability to pull and run `debian:stable-slim` for the container-backed simulated VM

## Project Structure

```text
xman/
├── app.go
├── main.go
├── cmd/
│   └── vcsim/
│       ├── demo_tree.go        # richer local simulator hierarchy seeding
│       └── main.go
├── internal/
│   ├── config/
│   │   ├── guestcredentials.go # guest credential metadata + keyring-backed password storage
│   │   ├── paths.go            # app config root resolution
│   │   ├── scripts.go          # stored script catalog + CRUD under xman/scripts
│   │   └── sshkeys.go          # SSH keypair storage and metadata
│   ├── jobs/                   # Background jobs, progress events, cancellation
│   ├── manager/                # Backend-agnostic Wails-facing feature layer
│   │   ├── backend.go
│   │   ├── commandlabel.go
│   │   ├── filetransfer.go
│   │   ├── guestcredentials.go
│   │   ├── guestexec.go
│   │   ├── networks.go
│   │   ├── snapshots.go
│   │   ├── ssh.go
│   │   ├── sshkeys.go
│   │   ├── tools.go
│   │   ├── vmconfig.go
│   │   ├── vmnetwork.go
│   │   └── vminfo.go
│   ├── sshtransport/
│   │   ├── deploykey.go        # Password bootstrap for key deployment
│   │   └── ssh.go              # SSH / SFTP transport
│   ├── vcenter/
│   └── workstation/
├── frontend/
│   ├── src/components/
│   │   ├── JobsBar.tsx
│   │   ├── MainView.tsx
│   │   └── features/
│   │       ├── sshkeys/        # SSH keys, guest credentials, stored scripts
│   │       ├── networks/
│   │       ├── inventory/
│   │       └── vms/
│   └── wailsjs/                # Generated Wails bindings
└── build/                      # Wails packaging assets
```

## Commit-Readiness Notes

Before check-in, a healthy local validation pass is:

```bash
make test-all
```

If you prefer the raw commands, the equivalent is:

```bash
GOCACHE=/tmp/gocache GOTMPDIR=/tmp go test -count=1 ./...
cd frontend && npm run build
```

If you are touching packaging or generated bindings, also verify:

```bash
make build-windows
```

## Testing Roadmap

Short-term backend coverage that is worth adding and maintaining:

- `vCenter + vcsim`: connection/authentication, VM listing/details, power operations, snapshots, hosts, datastores, and network inventory
- `Workstation + fake vmrun`: VM listing/details, power commands, snapshots, transfer flows, and guest command result handling without requiring a real VMware install
- `Workstation + opt-in host integration`: smoke tests against a real `vmrun` and VM directory when running from WSL against a Windows host
- `vCenter + vcsim + Docker`: stable guest command success coverage, guest file transfer, and probe coverage for cancellation / non-zero exit behavior against container-backed simulated VMs
- `Manager` lifecycle tests: backend swap, disconnect, and connection-scoped job cancellation
- Frontend state tests: jobs tracking, VM refresh coordination, and command-tab behavior around replace-vs-append output

## Future Work / TODO

### Command Execution
- Improve Workstation command cancellation so it attempts to kill the guest-side process, not just interrupt local waiting
- Add exit code / backend / run timestamp metadata directly in the command pane

### SSH / Security
- Add SSH host key trust and verification instead of `InsecureIgnoreHostKey`
- Add a lightweight “verify key access” action for stored SSH keys
- Allow optional per-run username override for advanced cases without changing the key default user

### VM Refresh / Performance
- Add adaptive polling so idle VM lists poll less aggressively than active ones
- Continue refining Workstation perf logging into a more structured trace format
- Consider fetching only lightweight VM browser fields until a VM is selected

### Guest Operations
- Guest file browser for navigating directories instead of typing full paths
- Guest process viewer / kill support
- Pull common guest logs in one click

### VM Lifecycle
- Clone / deploy from template
- Bulk power and snapshot actions
- Scheduled snapshots

### Multi-Connection
- Connect to multiple backends simultaneously in one app session

### Testing Coverage
- Expand the new `vCenter` `vcsim` suite to cover more negative paths and simulator fault injection
- Continue refining the optional `vcsim + Docker` guest-ops suite in CI or WSL2 with Docker, especially if simulator support for cancellation or exit codes improves
- Add manager-level lifecycle tests for reconnect/disconnect/job cancellation semantics
- Add manager-level shared tests for more guest-op/file/install edge cases as they appear in real bugs
- Add frontend tests for jobs UI, command output replacement, and backend-aware VM tab behavior
- Maintain a small real-`vCenter` manual checklist for VMware Tools, Windows guest ops, and privilege-sensitive paths
