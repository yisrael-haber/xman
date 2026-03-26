# xman

Using govmomi and a native UI to provide simple management of day-to-day tasks with VMware.

## Prerequisites

### Go

Version 1.21 or later required. [Download Go](https://go.dev/dl/)

```bash
go version
```

### Node.js

Version 18 or later required (used by Wails to build the frontend).

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

> **Note for Ubuntu 22.04 or older**: replace `libwebkit2gtk-4.1-dev` with `libwebkit2gtk-4.0-dev`.

`libsecret-1-dev` is required to build the keyring integration. `gnome-keyring` provides the Secret Service daemon at runtime (used to securely store saved passwords). On a full desktop Linux install these are typically already present.

### Wails CLI

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

After installing, verify all dependencies are satisfied:

```bash
wails doctor
```

All items should show as installed before proceeding.

### WSL2 Note

Wails renders a native GUI window. On WSL2 this requires **WSLg**, which is included with Windows 11 by default. Verify it is active:

```bash
echo $DISPLAY  # should output something like :0
```

If `$DISPLAY` is empty, WSLg is not running. Ensure you are on Windows 11 and WSL2 is up to date (`wsl --update` from a Windows terminal).

## Getting Started

```bash
# Clone the repository
git clone <repo-url>
cd xman

# Install frontend dependencies
cd frontend && npm install && cd ..

# Run in development mode (live reload)
wails dev -tags webkit2_41

# Build a production binary
wails build -tags webkit2_41
```

> **Ubuntu 24.04 / webkit note**: Ubuntu 24.04 ships `webkit2gtk-4.1` rather than `4.0`.
> The `-tags webkit2_41` flag is required on this platform.

## Project Structure

```
xman/
├── app.go                    # Wails entry point — wires features together
├── main.go
├── internal/
│   ├── vcenter/              # govmomi session and connection management
│   ├── jobs/                 # Long-running task management and progress events
│   └── features/
│       ├── filetransfer/     # Upload/download files to/from VM guests
│       ├── packetcapture/    # Packet capture via guest operations
│       └── vminfo/           # VM listing, power state, tools status
└── frontend/                 # Web UI (Wails auto-generates Go bindings)
```

## Requirements for Guest Operations

File transfer and packet capture features use VMware's **Guest Operations API**, which requires:

- VMware Tools (or open-vm-tools) installed and running inside the target VM
- Guest OS credentials (username and password) for the VM
- The vCenter user account must have the **Guest Operations** privilege on the VM

To install open-vm-tools on Linux guests:

```bash
# Debian/Ubuntu
sudo apt install open-vm-tools

# RHEL/CentOS/Rocky
sudo yum install open-vm-tools
```

Windows guests require VMware Tools installed from the vCenter-mounted ISO (`Actions > Guest OS > Install VMware Tools` in the vSphere client).
