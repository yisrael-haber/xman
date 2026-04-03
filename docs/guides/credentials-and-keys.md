# Credentials and SSH Keys

## Goal

Understand when to use guest credentials versus SSH keys, and how the normal
bootstrap flow works in `xman`.

## Two Authentication Models

### Guest Credentials

Use guest credentials for VMware Guest Ops actions such as:

- running commands through Guest Ops
- uploading or downloading files through Guest Ops

Guest credentials are username/password pairs stored in `xman`.

### SSH Keys

Use SSH keys for SSH and SFTP actions such as:

- running commands over SSH
- transferring files over SFTP

SSH keys in `xman` include a default SSH username.

## Recommended Bootstrap Flow

1. Create a guest credential for the VM.
2. Use Guest Ops or password-based key deployment once.
3. Deploy an SSH public key into the guest.
4. Switch day-to-day command and file transfer workflows to SSH/SFTP.

This keeps Guest Ops available as a fallback while making regular operations
faster and easier to repeat.

## Deploy SSH Key

The VM `SSH Key` tab is the normal bootstrap path when the guest is reachable
over SSH but does not yet trust your key.

Typical flow:

1. Create an SSH key in the global `SSH Keys` screen.
2. Set a default SSH username on the key if you know it.
3. Open the target VM.
4. Go to `SSH Key`.
5. Enter host, port, username, and password.
6. Deploy the selected public key.

## When to Prefer Guest Ops

Prefer Guest Ops when:

- the guest has VMware Tools but no SSH service yet
- the guest is not reachable on the network from your desktop
- you are doing first-boot or recovery work

## When to Prefer SSH

Prefer SSH when:

- the guest already has network reachability
- you expect to run repeated commands
- you want a simpler file transfer model

## Unexpected Behavior Sources

- `Deploy SSH Key` is intentionally the one SSH flow that still uses a password.
- Guest credentials are for Guest Ops. They are not reused automatically for
  SSH.
- Future SSH and SFTP operations use the key's default user, so an incorrect
  default user causes confusing connection failures later.
- Deleting or renaming a key affects later SSH and SFTP flows that depend on it.

## Verification

You should be able to:

- select a stored guest credential in VM tabs that use Guest Ops
- select an SSH key and see a default user available for SSH/SFTP flows

## Common Mistakes

- creating an SSH key without a default user
- expecting Guest Ops credentials to work for SSH
- expecting SSH keys to work before the public key has been deployed into the
  guest
