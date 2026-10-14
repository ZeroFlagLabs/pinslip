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

### Windows note data-loss fix (r2)

Fixed a bug where changing only the capitalisation of a note title
could delete the underlying Markdown file on Windows.

- Check whether old and new paths refer to the same physical file.
- Avoid deleting the newly saved file during case-only title changes.
- Add a regression test covering this behaviour.
- All Go tests pass.

Upstream bug report: https://github.com/homerious/pinslip/issues/20

Upstream pull request: https://github.com/homerious/pinslip/pull/21

### Windows login taskbar focus fix (r3)

Fixed an issue where restored notes triggered red/orange taskbar
attention indicators when PinSlip started at Windows login.

- Restored notes now use Electron's showInactive() method.
- Notes appear without requesting foreground focus.
- Newly created and manually opened notes retain normal focus behaviour.
- Confirmed working after a Windows reboot.
- TypeScript validation and application build pass.

## Base Version

PinSlip v1.0.1

Modified branch:

`modified-launch-behaviour`

Original modified release tag:

`modified-v1.0.1`

## Original installer

`PinSlip-Setup-1.0.1-Modified.exe`

SHA-256:

`B392EB1D0F78C7E00A5E9FB4FBDB19BB5F91103FDB3E34DD3DDBBD8CFE43605C`

The installer has been tested on multiple Windows PCs.

## Corrected installer (r2)

Release tag: `modified-v1.0.1-r2`

Installer: `PinSlip-Setup-1.0.1-Modified-r2.exe`

SHA-256:

`4AA3FE64AA813A77A156892951E482565C3C6B3C529216323FEBBD09F9FF051A`

This release includes the original modifications and the Windows
case-only title data-loss fix.

## Corrected installer (r3)

Release tag: `modified-v1.0.1-r3`

Installer: `PinSlip-Setup-1.0.1-Modified-r3.exe`

SHA-256:

`13FD1B08161850D241C8759F544BBEB2B2221E0D5B6336CEB984B894D755F18A`

This release includes all previous modifications, the r2 data-loss
fix and the Windows login taskbar focus fix.

## Modified Files

The modified build changes these files:

- `apps/desktop/src/main/index.ts`
- `apps/desktop/src/main/updater.ts`
- `apps/desktop/src/main/windows/note-window.ts`
- `apps/desktop/src/main/windows/window-manager.ts`
- `apps/service/internal/storage/engine.go`
- `apps/service/internal/storage/engine_test.go`

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