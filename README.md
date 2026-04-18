# DankPinentry

A pinentry implementation that displays a native [DankMaterialShell](https://github.com/AvengeMedia/DankMaterialShell) modal for passphrase entry. Works with GPG, SSH, and RBW (Bitwarden CLI).

## How it works

There are two parts that work together:
- **pinentry-dms**: Go binary that speaks the Assuan protocol on stdin/stdout and communicates with DMS via IPC
- **DMS plugin**: Daemon plugin that shows a themed FloatingWindow for passphrase entry

Pinentry client (gpg-agent or rbw) starts `pinentry-dms` and speaks to it with Assuan.
When the client runs `GETPIN` (to request the password), `pinentry-dms` opens a Unix socket
then sends an IPC call with the `dms` command to the DankPinentry plugin to show the modal
requesting the password. Once the user enters the password, the plugin writes it to the socket
which the `pinentry-dms` command picks up and prints for the pinentry client.

## Installation

### Home Manager (NixOS)

Add the flake input and configure the plugin via the DMS home-manager module:

```nix
# flake.nix inputs
dank-pinentry.url = "github:pacman99/DankPinentry";
```

```nix
# home-manager configuration
programs.dank-material-shell.plugins.dankPinentry = {
  enable = true;
  src = inputs.dank-pinentry.packages.${system}.dms-plugin;
};
```

### Manual

Symlink or copy the `plugin/` directory into your DMS plugins folder:

```bash
ln -s /path/to/DankPinentry/plugin ~/.config/DankMaterialShell/plugins/dankPinentry
```

Then enable the plugin in DMS settings.

### Pinentry binary

Build with Nix:

```bash
nix build .#pinentry-dms
```

Or with Go:

```bash
CGO_ENABLED=0 go build ./cmd/pinentry-dms/
```

## Configuration

Point your GPG agent or RBW to the binary:

**RBW** (`~/.config/rbw/config.json`):
```json
{
  "pinentry": "/path/to/pinentry-dms"
}
```

**GPG** (`~/.gnupg/gpg-agent.conf`):
```
pinentry-program /path/to/pinentry-dms
```

## Requirements

- DankMaterialShell with the dankPinentry plugin enabled
- `dms` binary on PATH (for IPC)
