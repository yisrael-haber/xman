# Transferring Files

## Goal

Move files between the host running `xman` and a guest VM.

## Current Scope

The built-in transfer workflow is currently file-oriented:

- upload a file
- download a file

Directory transfer is not yet a first-class feature in the UI.

## Available Transports

### Guest Ops

Good for:

- guests that have VMware Tools but no SSH yet
- environments where desktop-to-guest network reachability is limited

### SSH / SFTP

Good for:

- guests with working SSH access
- repeated transfers after an initial bootstrap

## Upload a File

1. Open the VM `Files` tab.
2. Choose the transport.
3. Select a local file.
4. Enter the guest destination path.
5. Start the upload job.

## Download a File

1. Open the VM `Files` tab.
2. Choose the transport.
3. Enter the guest source path.
4. Choose the local destination path.
5. Start the download job.

## Path Examples

Linux guest paths:

- `/tmp/setup.sh`
- `/opt/packages/python.tar.gz`

Windows guest paths:

- `C:\Users\Public\installer.exe`
- `C:\Temp\packages.zip`

## Recommended Practices

- Put temporary installer content under a predictable staging directory.
- Use filenames that make job logs easy to read.
- Prefer SSH/SFTP once the guest is fully bootstrapped.

## Unexpected Behavior Sources

- Wrong Windows versus POSIX path syntax is one of the most common transfer
  failures.
- Guest Ops transfer can fail even on a powered-on guest if VMware Tools is
  still starting.
- A transfer that works over SSH may still fail over Guest Ops if guest
  credentials or guest permissions are wrong.
- Offline bundle workflows often use file transfer plus a separate unpack step,
  not a single "transfer directory" action.
