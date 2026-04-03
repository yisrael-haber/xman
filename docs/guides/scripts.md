# Stored Scripts

## Goal

Manage reusable scripts in one place and run them later from the VM `Run` tab.

## Main Actions

In the global `Scripts` feature you can:

- create a new script
- edit an existing script
- delete a script
- refresh the script catalog

Scripts are stored as regular files under the app config directory and become
available from the VM `Run` tab.

## Script Kinds

Script kind is inferred from the filename:

- `.sh` for POSIX guests
- `.cmd` and `.bat` for Windows guests
- `.txt` for generic snippets

## Recommended Usage

Use stored scripts for:

- repeatable bootstrap tasks
- package install snippets
- environment validation commands
- cleanup and verification steps

## Unexpected Behavior Sources

- Stored `.ps1` files can be saved, but they are not supported in the current
  `Run` tab flow.
- Scripts are global reusable assets, not per-VM state.
- The app prompts before discarding unsaved script edits.
