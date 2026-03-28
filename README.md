# xman

`xman` is a native desktop application for day-to-day VMware management. It is built with Go and Wails and currently supports both **vCenter** and **VMware Workstation** backends.

The app is meant to make common operator workflows fast:
- browse VMs
- power-cycle them
- run commands
- transfer files
- deploy SSH keys
- install packages remotely
- inspect inventory and networks

## Highlights

### VM Management
- Browse and select VMs with power state, VMware Tools state, IP address, CPU, and memory
- Power on, power off, reset, and suspend VMs
- Keep job completion and VM state refresh synchronized

### Snapshots
- List snapshots with creation time and tree depth
- Create snapshots with optional description, memory capture, and quiesce support
- Revert to or delete snapshots

### Run Command
Run commands inside a guest VM using two transports:

- **VMware Guest Operations**: requires VMware Tools plus guest username/password
- **SSH**: uses `host + SSH key` only; the SSH username comes from the selected key's default user

Run Command is intentionally modeled as **separate shell sessions**, not a live terminal. Each new command replaces the previous output in the command pane.

It also supports:
- copy output to clipboard
- cancel a running command without restarting the app
- launch a real interactive SSH shell in the native OS terminal
- background job tracking with detailed logs

### File Transfer
Upload and download files to or from a guest VM using:

- **VMware Guest Operations**
- **SSH / SFTP**

For SSH / SFTP, the app uses a stored private key plus that key's default user. Guest passwords are not used for day-to-day SSH transfers.

### Remote Install
Upload an installer package to the guest and execute it silently. Supports:

| Package type | Installer invoked |
|---|---|
| `.msi` | `msiexec /i ... /qn /norestart` |
| `.msix` / `.msixbundle` | `powershell Add-AppxPackage` |
| `.exe` | silent flags (`/S /SILENT /VERYSILENT /quiet`) |
| `.deb` | `dpkg -i` + `apt-get install -f` |
| `.rpm` | `rpm -i` |

The detected install command is editable before execution.

### SSH Keys
- Generate SSH key pairs inside the app
- Store key metadata including a default SSH username
- Deploy the public key to a guest using a one-time password bootstrap flow
- Use deployed keys for SSH / SFTP afterward without re-entering passwords

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

### Inventory
Available on **vCenter**:
- ESXi host inventory with CPU and memory utilization
- datastore inventory with capacity and free space

### VMware Tools Management
- Install or upgrade VMware Tools on Windows guests
- On vCenter: trigger built-in Tools upgrade
- On Workstation: mount the bundled Tools ISO and start installation

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
| Power operations         | ✓       | ✓           |
| Snapshots                | ✓       | ✓           |
| Run Command (VMware)     | ✓       | ✓           |
| Run Command (SSH)        | ✓       | ✓           |
| File Transfer (VMware)   | ✓       | ✓           |
| File Transfer (SSH/SFTP) | ✓       | ✓           |
| Remote Install           | ✓       | ✓           |
| Deploy SSH Key           | ✓       | ✓           |
| Networks view            | ✓       | ✓           |
| Host inventory           | ✓       | —           |
| Datastore inventory      | ✓       | —           |
| Tools auto-upgrade       | ✓       | —           |
| Tools ISO install        | —       | ✓ (Windows) |

## How SSH Works

The app now uses a lightweight SSH model:

1. Create an SSH key pair in the **SSH Keys** panel
2. Set a default user on that key
3. Use **Deploy SSH Key** once to push the public key to the guest with a password
4. After that, Run Command / File Transfer / Remote Install over SSH use:
   - `host`
   - `key`
   - the key's default user

For a true live shell, the Run Command tab can also launch the native OS terminal with the system `ssh` client and the selected private key.

This avoids keeping per-VM deployment state in local config and keeps the runtime behavior predictable:
- if the selected key works for `user@host`, SSH succeeds
- if it does not, SSH fails cleanly

## VMware Guest Operations Requirements

VMware transport for Run Command, File Transfer, and Remote Install requires:
- VMware Tools or open-vm-tools installed and running
- guest OS credentials
- on vCenter, guest operations privileges on the VM

Linux guests can usually use `open-vm-tools`:

```bash
# Debian / Ubuntu
sudo apt install open-vm-tools

# RHEL / CentOS / Rocky
sudo yum install open-vm-tools
```

If VMware Tools are not available, switch to the SSH-based transport instead.

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
make vet
```

There is also a `vcsim` helper target for local vCenter-style simulator work:

```bash
make vcsim
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

## Project Structure

```text
xman/
├── app.go
├── main.go
├── internal/
│   ├── config/
│   │   └── sshkeys.go          # SSH keypair storage and metadata
│   ├── jobs/                   # Background jobs, progress events, cancellation
│   ├── manager/                # Backend-agnostic Wails-facing feature layer
│   │   ├── backend.go
│   │   ├── filetransfer.go
│   │   ├── guestexec.go
│   │   ├── install.go
│   │   ├── snapshots.go
│   │   ├── ssh.go
│   │   ├── sshkeys.go
│   │   ├── tools.go
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
│   │       ├── sshkeys/
│   │       └── vms/
│   └── wailsjs/                # Generated Wails bindings
└── build/                      # Wails packaging assets
```

## Commit-Readiness Notes

Before check-in, a healthy local validation pass is:

```bash
GOCACHE=/tmp/gocache GOTMPDIR=/tmp go test ./...
cd frontend && npm run build
```

If you are touching packaging or generated bindings, also verify:

```bash
make build-windows
```

## Future Work / TODO

### Command Execution
- Improve Workstation command cancellation so it attempts to kill the guest-side process, not just interrupt local waiting
- Add exit code / backend / run timestamp metadata directly in the command pane
- Consider optional saved command snippets for repeated operator workflows

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
