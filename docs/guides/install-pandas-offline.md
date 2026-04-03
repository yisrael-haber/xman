# Install Pandas Offline

## Goal

Install `pandas` on a guest that does not have internet access.

The recommended pattern is:

1. build a local wheel bundle on an internet-connected machine
2. package it
3. upload it to the guest
4. install from local files only

## Requirements

- Python is already installed on the guest
- you can run commands in the guest through Guest Ops or SSH
- you have a host machine with internet access to prepare the package bundle

## Prepare a Wheelhouse on the Host

On an internet-connected machine, create a folder and download the required
packages:

```bash
mkdir -p wheelhouse
python -m pip download --dest wheelhouse pandas
```

This downloads `pandas` and its dependencies into `wheelhouse`.

## Package the Wheelhouse

Create a tar archive on the host:

```bash
tar -czf pandas-wheelhouse.tgz wheelhouse
```

## Upload the Archive

Use the `Files` tab in `xman` to upload the archive to the guest.

Example guest destination:

- Linux: `/tmp/pandas-wheelhouse.tgz`
- Windows: `C:\Users\Public\pandas-wheelhouse.tgz`

## Extract the Archive on the Guest

### Linux guest

```bash
mkdir -p /tmp/pandas-wheelhouse
tar -xzf /tmp/pandas-wheelhouse.tgz -C /tmp/pandas-wheelhouse
```

### Windows guest

If the guest already has `tar.exe` available:

```powershell
mkdir C:\Users\Public\pandas-wheelhouse
tar -xzf C:\Users\Public\pandas-wheelhouse.tgz -C C:\Users\Public\pandas-wheelhouse
```

If `tar.exe` is not available, use a zip-based bundle instead of `.tgz` for
the Windows version of this workflow.

## Install Pandas Without Internet

### Linux guest

```bash
python3 -m pip install --no-index --find-links /tmp/pandas-wheelhouse/wheelhouse pandas
```

### Windows guest

```powershell
python -m pip install --no-index --find-links C:\Users\Public\pandas-wheelhouse\wheelhouse pandas
```

## Verify the Install

Run:

```bash
python -c "import pandas; print(pandas.__version__)"
```

Or on Windows:

```powershell
python -c "import pandas; print(pandas.__version__)"
```

## Notes

- This guide uses a tar archive because it is convenient for moving many files.
- The guest never needs internet connectivity.
- The same wheelhouse pattern works for many other Python packages.

## Future Expansion

Possible follow-up guides:

- offline virtual environment creation
- offline install of a private internal package
- Linux-specific wheelhouse creation for another architecture
