# Browser Console

## Goal

Open the VMware HTML5 browser console for a `vCenter` VM and understand what
the console diagnostics mean.

## Availability

The browser console is available on the `vCenter` backend only.

## Main Flow

1. Open a VM.
2. Go to the `Console` tab.
3. Review the diagnostics.
4. Click `Open Console`.

## What the Diagnostics Show

The console screen surfaces:

- the vCenter URL
- the connected host
- the reported console host
- the host-selection source
- a redacted launch URL
- simple reachability checks

## Unexpected Behavior Sources

- Console launch does not depend on the guest IP address.
- It does depend on desktop reachability to both vCenter and the selected
  console host.
- Each launch uses a fresh one-time ticket, so old console URLs are not meant
  to be reused.
- A hostname mismatch between the connected endpoint and the reported console
  host is often a DNS or routing clue rather than an xman bug.
