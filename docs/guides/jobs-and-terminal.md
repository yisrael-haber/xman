# Jobs and Terminal Output

## Goal

Understand how `xman` reports progress, logs, and completion for long-running
operations.

## What Jobs Track

Jobs are used for operations such as:

- command execution
- file transfer
- snapshot actions
- power actions
- key deployment
- VMware Tools actions

## What You Can Do

From the job UI you can:

- watch progress
- inspect log messages
- see final success or failure state
- dismiss completed jobs
- cancel running command jobs

## Terminal-Like Output

The `Run` tab and the job system work together:

- command output is captured as job data
- the final result is shown in the `Run` pane
- non-zero exit states are surfaced in a readable way

## Unexpected Behavior Sources

- The in-app command view is captured output, not a live terminal stream.
- Starting another run replaces the previous in-app output pane.
- Completed jobs stay in memory until dismissed.
- Cancellation is most reliable and useful for command execution jobs.
