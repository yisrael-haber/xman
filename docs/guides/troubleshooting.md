# Troubleshooting

## Goal

Collect the most common failure modes in one place so operators can quickly
recover when a workflow does not behave as expected.

## Guest Ops Is Not Ready

Symptoms:

- file transfer fails immediately
- command execution reports VMware Tools is not ready

Check:

- the VM is powered on
- VMware Tools or `open-vm-tools` is installed
- the tools service is running inside the guest

## SSH Key Problems

Symptoms:

- SSH/SFTP connect fails before the command starts

Check:

- the selected key has a default user
- the public key has already been deployed to the guest
- the guest is reachable on the expected SSH port

## Wrong Path Format

Symptoms:

- upload or download fails with file not found
- installer launch fails even though upload succeeded

Check:

- Windows paths use backslashes and a drive letter
- Linux paths use forward slashes
- the exact filename on the guest matches what was uploaded

## Silent Installer Fails

Symptoms:

- the command job exits non-zero
- the software does not appear after install

Check:

- the installer actually supports silent flags
- the install path is writable
- the staged installer path is correct

## Offline Python Package Install Fails

Symptoms:

- `pip` tries to reach the internet
- dependency resolution fails

Check:

- use `--no-index`
- point `--find-links` at the extracted wheelhouse directory
- confirm the wheelhouse contains `pandas` and all dependencies

## When in Doubt

If a workflow is failing in Guest Ops, try the same workflow over SSH.
If a workflow is failing over SSH, fall back to Guest Ops when VMware Tools is
available.

The two transport modes are often useful as cross-checks when diagnosing
environment issues.
