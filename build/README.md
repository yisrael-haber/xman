# Build Assets

This directory contains packaging assets and generated build output used by Wails.

## Layout

- `bin/` - compiled application binaries
- `darwin/` - macOS packaging assets
- `windows/` - Windows packaging assets
- `appicon.png` - source icon used to generate platform-specific icons

## Notes

- `bin/` is disposable build output
- the platform asset directories are safe to customize if you need installer or metadata changes
- if you want to reset platform packaging files back to Wails defaults, delete the customized files and rebuild with `wails build`
- from WSL, the main Windows packaging workflow is `make build-windows` or `make deploy-windows`
- the repo's test/build readiness checks live at the root `Makefile` and `README.md`; this directory only covers packaging assets

## Windows

The Windows directory contains the manifest, version metadata, and installer assets used during `wails build`.

Important files:

- `icon.ico`
- `info.json`
- `wails.exe.manifest`
- `installer/*`
