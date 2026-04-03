# Backend Differences

## Goal

Explain the practical differences between the `vCenter` and `Workstation`
backends so operators know what to expect before starting a workflow.

## vCenter

Best for:

- infrastructure managed by ESXi and vCenter
- inventory browsing across hosts and datastores
- browser console launch

Strengths:

- inventory features
- console support
- richer infrastructure context
- richer snapshot options

Tradeoffs:

- requires vCenter connectivity and permissions
- Guest Ops depends on VMware Tools readiness and guest permissions

## Workstation

Best for:

- local lab environments
- directly managed Workstation VMs
- fast iteration on a single desktop or lab machine

Strengths:

- simple local backend model
- good fit for homelabs and local testing

Tradeoffs:

- no host inventory or datastore views
- no browser console flow
- some workflows may depend more heavily on `vmrun` behavior
- snapshot options are simpler than on `vCenter`

## Transport Layer Still Matters

The backend and the transport are different decisions.

Examples:

- `vCenter` + Guest Ops
- `vCenter` + SSH
- `Workstation` + Guest Ops
- `Workstation` + SSH

Choose the backend based on where the VM is managed.
Choose the transport based on what access the guest currently has.

## Practical Differences That Surprise People

- `Inventory` and browser console are `vCenter` features only.
- Both backends support networks, but they describe different infrastructure.
- `Workstation` often has stricter power-state requirements for config edits.
