# Running Commands in a Guest

## Goal

Run one-off commands or stored scripts inside a guest VM using either Guest Ops
or SSH.

## Recommended Transport

Use Guest Ops when:

- SSH is not ready yet
- network reachability is limited

Use SSH when:

- SSH access already works
- you want the simplest repeated command workflow

## Run a Raw Command

1. Open a VM.
2. Go to the `Run` tab.
3. Choose the transport:
   - `VMware` for Guest Ops
   - `SSH` for SSH
4. Select the required credential or key.
5. Enter the command.
6. Start the job.

## Run a Stored Script

1. Create or select a saved script in the `Scripts` feature.
2. Open the VM `Run` tab.
3. Choose `Stored Script`.
4. Select the script.
5. Choose the transport.
6. Start the job.

## Other Useful Actions

The `Run` workflow also supports:

- copying captured output
- cancelling a running command job
- launching a real interactive SSH session in the host terminal

## Verification

Confirm:

- a job appears in the job list
- command output is captured
- the exit status is reflected in the final message

## Unexpected Behavior Sources

- The `Run` tab is not a live shell. Each run starts a separate session.
- Starting a new run replaces the previous in-app output pane.
- Stored `.ps1` files are not supported in the current `Run` tab execution
  model.
- On `Workstation`, Guest Ops command runs can feel slower because output is
  captured through guest-side temp files instead of a streamed shell.

## Tips

- Use Guest Ops for bootstrap steps such as enabling SSH.
- Use SSH for repeated administrative tasks after key deployment.
- Keep scripts small and composable so failures are easier to diagnose.
