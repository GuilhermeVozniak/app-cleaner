# App Cleaner CLI

Full-parity terminal port of App Cleaner's cleaning engine — same 16
scan categories, same safety tiers, same undo backups as the desktop
app, driven from a keyboard-only TUI or scriptable flags.

## Install

**Homebrew** (recommended):
```
brew install GuilhermeVozniak/tap/app-cleaner-cli
app-cleaner --version
```

**Download a release tarball**:
```
curl -LO https://github.com/GuilhermeVozniak/app-cleaner/releases/latest/download/app-cleaner-cli_<version>_darwin_universal.tar.gz
tar xzf app-cleaner-cli_<version>_darwin_universal.tar.gz
./app-cleaner --version
```

**Build from source**:
```
git clone https://github.com/GuilhermeVozniak/app-cleaner.git
cd app-cleaner
task build:cli
./apps/cli/bin/app-cleaner --version
```

## Commands

| Command | Behavior |
|---|---|
| *(none)* | Interactive mode: full scan with live progress, category picker, file picker for categories that support it, confirm (default yes), backup + clean, results |
| `scan [--categories c1,c2] [--json]` | Non-interactive scan; human summary or JSON |
| `clean [--categories…] [--dry-run] [--yes] [--backup/--no-backup] [--json]` | Non-interactive clean; `--dry-run` never prompts or deletes; without `--yes`, confirm defaults to **no** |
| `uninstall [--app name]` | App picker or direct by name; blocks on a running app; shows related paths before confirming |
| `maintenance --dns --purgeable --timemachine` | Sequential maintenance tasks with spinners; no flags prints a "no tasks specified" hint |
| `backups list\|restore <path>\|clean-old` | Manage the shared backup store — CLI and GUI backups are the same directory and appear in both |

## Interactive keymap

**Category picker** (checkbox list, page size 15):

| Key | Action |
|---|---|
| `space` | Toggle current category |
| `a` | Select/deselect all |
| `i` | Invert selection |
| `enter` | Confirm and continue |

**File picker** (dual-pane: categories left, files right — shown only for categories that support per-file selection):

| Key | Pane | Action |
|---|---|---|
| `space` | either | Toggle current row |
| `a` | either | Select/deselect all in scope |
| `i` | either | Invert selection in scope |
| `→` | categories | Enter files pane for the caret category |
| `d` | files | Toggle every file in the caret's directory |
| `m` | files | Expand caret's directory (+10 files) |
| `h` | files | Collapse caret's directory (back to 5) |
| `c` | files | Copy the caret file's directory path to the clipboard |
| `←` / `backspace` | files | Return to the categories pane |
| `enter` | either | Confirm and continue |

## Backups

Every clean (unless `--no-backup` or a category's safety default disables it)
moves items into `~/Library/Application Support/AppCleaner/Backups/<timestamp>/`
instead of deleting them outright — `backups restore <path>` undoes it. This
is the same store the desktop app reads and writes; a backup made from
either app is visible and restorable from the other.

## Elevation

Maintenance tasks that need admin rights (DNS flush, purgeable space,
Time Machine local snapshots) prompt via a native macOS admin dialog
(osascript), not `sudo` — the same mechanism as the desktop app, so
behavior is consistent whether you clean from the terminal or the GUI.
