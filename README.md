# xman

A native desktop application for day-to-day VMware management tasks. Built with Go and Wails, it supports both **vCenter** and **VMware Workstation** backends.

## Features

### VM Management
- Browse and select VMs with power state and VMware Tools status indicators
- Power on, power off, reset, and suspend VMs
- View VM details: CPU, memory, guest OS, IP address, tools version

### Snapshots
- List all snapshots with creation time and tree depth
- Create snapshots with optional description and quiesce support
- Revert to any snapshot
- Delete snapshots

### Run Command
Execute commands inside a guest VM using two transports:

- **VMware** — Uses the VMware Guest Operations API. Requires VMware Tools running in the guest. Supports Windows (via PowerShell) and Linux guests.
- **SSH** — Direct SSH connection to the guest. Useful when VMware Tools are not available (e.g. bootstrapping). Host auto-populates from the VM's reported IP address.

Output is captured and displayed inline. Commands are tracked as background jobs.

### File Transfer
Upload and download files to/from a guest VM using two transports:

- **VMware** — Uses the VMware Guest Operations API (requires VMware Tools)
- **SSH / SFTP** — Direct SFTP connection. Useful when VMware Tools are not available

Both directions (upload/download) share the same credentials and transport selection.

### Inventory (vCenter only)
- List ESXi hosts with CPU and memory utilization
- List datastores with capacity and free space

### VMware Tools Management
- Install or upgrade VMware Tools on Windows guests
- On vCenter: triggers the built-in auto-upgrade mechanism
- On Workstation: mounts the bundled Tools ISO and runs the installer
- open-vm-tools is recommended for Linux guests (see below)

### Job Tracking
All long-running operations (file transfers, command execution, Tools install) are tracked as jobs with progress indication, log output, and status. Jobs can be dismissed individually or all at once.

---

## Prerequisites

### Go

Version 1.21 or later. [Download Go](https://go.dev/dl/)

```bash
go version
```

### Node.js

Version 18 or later (used by Wails to build the frontend).

```bash
# Install via NodeSource (Ubuntu/Debian)
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt install -y nodejs

# Verify
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

> **Ubuntu 22.04 or older**: replace `libwebkit2gtk-4.1-dev` with `libwebkit2gtk-4.0-dev`.

`libsecret-1-dev` is required to build the keyring integration. `gnome-keyring` provides the Secret Service daemon used to securely store saved passwords.

### Wails CLI

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails doctor
```

All items should show as installed before proceeding.

### WSL2 Note

Wails renders a native GUI window. On WSL2 this requires **WSLg**, included with Windows 11 by default.

```bash
echo $DISPLAY  # should output something like :0
```

If `$DISPLAY` is empty, WSLg is not running. Run `wsl --update` from a Windows terminal and ensure you are on Windows 11.

---

## Getting Started

```bash
git clone <repo-url>
cd xman

# Install frontend dependencies
cd frontend && npm install && cd ..

# Run in development mode (live reload)
wails dev -tags webkit2_41

# Build a production binary
wails build -tags webkit2_41
```

> **Ubuntu 24.04**: the `-tags webkit2_41` flag is required because Ubuntu 24.04 ships `webkit2gtk-4.1`.

---

## Project Structure

```
xman/
├── app.go                       # Wails entry point
├── main.go
├── internal/
│   ├── config/                  # Connection settings and credential storage
│   ├── jobs/                    # Background job tracking and progress events
│   ├── manager/                 # Backend-agnostic feature layer (Wails bindings)
│   │   ├── backend.go           # Backend interface
│   │   ├── vminfo.go            # VM listing and power operations
│   │   ├── snapshots.go         # Snapshot management
│   │   ├── filetransfer.go      # VMware guest file transfer
│   │   ├── guestexec.go         # Guest command execution
│   │   ├── ssh.go               # SSH/SFTP transport bindings
│   │   ├── psutil.go            # PowerShell helpers for Windows guests
│   │   ├── inventory.go         # Host and datastore inventory
│   │   └── tools.go             # VMware Tools installation
│   ├── vcenter/                 # vCenter backend (govmomi)
│   ├── workstation/             # VMware Workstation backend (vmrun CLI)
│   └── sshtransport/            # SSH/SFTP client implementation
└── frontend/                    # Web UI (React + Vite, Wails auto-generates Go bindings)
```

---

## Backend Comparison

| Feature                  | vCenter | Workstation |
|--------------------------|:-------:|:-----------:|
| VM listing               | ✓       | ✓           |
| Power operations         | ✓       | ✓           |
| Snapshots                | ✓       | ✓           |
| Run Command (VMware)     | ✓       | ✓           |
| File Transfer (VMware)   | ✓       | ✓           |
| Run Command (SSH)        | ✓       | ✓           |
| File Transfer (SSH/SFTP) | ✓       | ✓           |
| Host inventory           | ✓       | —           |
| Datastore inventory      | ✓       | —           |
| Tools auto-upgrade       | ✓       | —           |
| Tools ISO install        | —       | ✓ (Windows) |

---

## Guest Operations Requirements

VMware transport (Run Command and File Transfer) uses the **VMware Guest Operations API**, which requires:

- VMware Tools (or open-vm-tools) installed and running inside the guest
- Guest OS credentials (username and password)
- For vCenter: the user account must have the **Guest Operations** privilege on the VM

To install open-vm-tools on Linux guests:

```bash
# Debian/Ubuntu
sudo apt install open-vm-tools

# RHEL/CentOS/Rocky
sudo yum install open-vm-tools
```

Windows guests require VMware Tools installed from the vCenter ISO (`Actions > Guest OS > Install VMware Tools`) or via the Workstation Tools install feature in this application.

If VMware Tools are not available, switch to **SSH / SFTP** transport in the Run Command or File Transfer tabs.

---

## Roadmap

### Network Management
- **Guest network configuration** — view and change IP address, subnet mask, gateway, and DNS settings inside a running guest (via guest exec or SSH); useful for reconfiguring a cloned VM without needing to open a console
- **vNIC management** — add, remove, or reconnect virtual network adapters; change the portgroup/network a VM is connected to
- **Port forwarding (Workstation)** — manage VMware Workstation NAT port-forwarding rules from the UI

### Packet Capture / Network Sniffing
- **Guest-side capture** — run `tcpdump` or `Wireshark` inside the guest via SSH or guest exec, stream the output to a local `.pcap` file; works on both backends
- **vCenter distributed switch capture** — use the vSphere port mirroring or `dvs.VmVnic.Capture` API to capture traffic on a VM's vNIC at the hypervisor level without touching the guest OS; vCenter only

### Guest Inspection
- **File browser** — browse the guest filesystem directory tree and open/download individual files without specifying a full path each time
- **Process viewer** — list running processes inside the guest and send kill signals; useful for troubleshooting hung services
- **Log harvester** — select common log locations (`/var/log`, Windows Event Log) and pull them to the local machine in one click

### VM Lifecycle
- **Clone / deploy from template** — clone a running VM or deploy a new one from a template with configurable name, datastore, and network (vCenter)
- **Bulk operations** — apply a power action or snapshot to multiple selected VMs at once
- **Scheduled snapshots** — configure a recurring snapshot schedule (e.g. nightly before maintenance)

### Multi-Connection
- **Multiple simultaneous backends** — connect to more than one vCenter or Workstation instance in the same session and switch between them without disconnecting
- **Saved connection profiles** — store named connection profiles (URL, username, backend type) so reconnecting is a single click

### Credential & Access Helpers
- **SSH key deployment** — generate or import an SSH key pair and push the public key to `~/.ssh/authorized_keys` inside the guest via VMware guest ops, eliminating the need to re-enter passwords for SSH transport
- **Credential profiles** — save named guest credential sets and select them from a dropdown rather than typing on each operation
