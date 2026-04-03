# Install Python Offline

## Goal

Install Python on a guest without relying on guest internet access.

This guide focuses on the common Windows workflow:

- stage the Python installer on the host running `xman`
- upload it to the guest
- run the installer silently from the command line

## Requirements

- a Windows guest VM
- either Guest Ops credentials or SSH access
- a Python installer already downloaded to the host

Example installer filename:

- `python-3.12.8-amd64.exe`

## Recommended Transport

Use Guest Ops when:

- the guest is reachable through VMware Tools but not yet through SSH

Use SSH when:

- SSH access already works and you prefer to stay in one transport

## Stage the Installer

Download the Python installer on the host that is running `xman`.
Keep it in an easy-to-find folder such as:

- `~/Downloads/python-3.12.8-amd64.exe`

## Upload the Installer

In `xman`:

1. Open the target VM.
2. Go to `Files`.
3. Upload the installer to a stable staging path such as:
   - `C:\Users\Public\python-3.12.8-amd64.exe`

## Run the Installer Silently

Open the `Run` tab and execute a command such as:

```powershell
C:\Users\Public\python-3.12.8-amd64.exe /quiet InstallAllUsers=1 PrependPath=1 Include_test=0
```

If you prefer to avoid modifying `PATH` during the install, remove
`PrependPath=1`.

## Verify the Install

Run one or more of:

```powershell
python --version
py --version
where python
```

If `python` is not on `PATH`, try the common install location:

```powershell
C:\Program Files\Python312\python.exe --version
```

## Cleanup

Optionally remove the staged installer:

```powershell
del C:\Users\Public\python-3.12.8-amd64.exe
```

## Troubleshooting

- If the installer does not start, confirm the path is correct.
- If the job exits non-zero, rerun with a simpler flag set and verify the
  installer build supports silent install flags.
- If `python` is installed but not found, use the full install path or reopen
  the session after the `PATH` change.
