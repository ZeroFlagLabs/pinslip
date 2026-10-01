# PinSlip Modified Build

This branch is based on PinSlip v1.0.1 by the original PinSlip contributors.

The aim of this modified build is to retain the original PinSlip appearance
and functionality while changing a small amount of startup behaviour.

## Changes from PinSlip v1.0.1

### Startup behaviour

The original PinSlip opens the manager window when the application launches.

This modified build behaves more like Windows Sticky Notes:

- If one or more sticky notes were open when PinSlip last exited, those notes
  are restored and the manager remains hidden.
- If no sticky notes were open when PinSlip last exited, the manager opens
  normally.
- Launching PinSlip again while it is already running does not force the
  manager open when sticky notes are already open.
- The manager can still be opened at any time from the system tray.
- If the system tray icon is disabled, the manager is shown so the application
  cannot start invisibly.

### Automatic updates

Automatic updating is disabled in this modified build.

This prevents an official PinSlip update from automatically replacing the
modified installation.

- No automatic update check at startup.
- No automatic update download.
- No automatic installation on exit.
- Manual **Check for updates** displays:

  `Updates are disabled in this modified build.`

The original download-page button remains available for manually checking
official PinSlip releases.

## Base Version

PinSlip v1.0.1

Modified branch:

`modified-launch-behaviour`

Modified release tag:

`modified-v1.0.1`

## Installer

`PinSlip-Setup-1.0.1-Modified.exe`

SHA-256:

`B392EB1D0F78C7E00A5E9FB4FBDB19BB5F91103FDB3E34DD3DDBBD8CFE43605C`

The installer has been tested on multiple Windows PCs.

## Modified Files

Application behaviour changes are contained in:

- `apps/desktop/src/main/index.ts`
- `apps/desktop/src/main/updater.ts`

The local `Modified Release` directory is excluded from Git.

## Upstream Project

This is a modified fork of PinSlip.

Original project:

`homerious/pinslip`

The original project remains configured locally as the `upstream` Git remote
so future PinSlip releases can be reviewed and incorporated if desired.

## Licence

PinSlip is distributed under the MIT License.

The original copyright and licence notice are retained.