# App Cleaner

A native macOS cleaning app — scan and remove caches, logs, old downloads, orphaned
`node_modules`, duplicate files and more; uninstall apps with their leftovers; run
system maintenance; undo cleans from move-based backups. 100% offline, no telemetry.

Go engine + [Wails v2](https://wails.io) shell + React/TypeScript UI. A desktop port of
[mac-cleaner-cli](https://github.com/guhcostan/mac-cleaner-cli) with full feature parity.

<!-- Screenshot: once available, add a capture of the Smart Scan view here, e.g. ![App Cleaner — Smart Scan](docs/screenshot.png) -->

## Features — 16 scan categories

| Category | Group | Safety | What it cleans |
|---|---|---|---|
| Temporary Files | System Junk | 🟢 safe | `/tmp` and `/var/folders` temp files |
| User Cache Files | System Junk | 🟡 moderate | Application caches in `~/Library/Caches` |
| System Log Files | System Junk | 🟡 moderate | System and application logs |
| Orphaned Launch Agents | System Junk | 🟡 moderate | Launch agents pointing to non-existent apps |
| Language Files | System Junk | 🔴 risky | Unused `.lproj` localizations in `/Applications` |
| Homebrew Cache | Development | 🟢 safe | Homebrew download cache and old versions |
| Docker | Development | 🟢 safe | Unused images, containers and build cache |
| Development Cache | Development | 🟡 moderate | npm, yarn, pnpm, pip, CocoaPods, Gradle, Cargo, Xcode DerivedData/Archives |
| Node Modules | Development | 🟡 moderate | `node_modules` in old (30d+) or orphaned projects |
| Trash | Storage | 🟢 safe | Files in the Trash bin |
| Old Downloads | Storage | 🔴 risky | Downloads older than N days (default 30) |
| iOS Backups | Storage | 🔴 risky | iPhone/iPad backups (MobileSync) |
| Mail Attachments | Storage | 🔴 risky | Mail.app downloaded attachments |
| Duplicate Files | Storage | 🔴 risky | Identical files (keeps the newest copy) |
| Browser Cache | Browsers | 🟢 safe | Chrome, Safari, Firefox and Arc caches |
| Large Files | Large Files | 🔴 risky | Files ≥ 500 MB in Downloads/Documents, for review |

Plus: **App Uninstaller** (bundle + 11 kinds of leftovers, running-app guard),
**Maintenance** (flush DNS, free purgeable space, clear Time Machine snapshots),
**Backups/Undo** (move-based, restorable, auto-pruned), **Settings**.

Safe+moderate categories are pre-selected after a scan; risky ones never are. Deletion
refuses system paths (`/System`, `/usr`, …) by design. Grant the app **Full Disk Access**
for Trash/Safari/Mail categories.

## Build

Requirements: macOS 11+, Go 1.26+, Node 18+, Wails CLI v2.13 (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0`).

```bash
wails doctor                             # verify toolchain
wails dev                                # live-reload development
wails build -clean                              # build/bin/App Cleaner.app (host arch, dev iterations)
wails build -clean -platform darwin/universal   # universal release build
```

## Testing

```bash
go test ./...                                  # engine + bridge
cd frontend && npm run typecheck && npm test   # tsc --noEmit + vitest
```

## Signing & notarization (follow-up — the app currently ships unsigned)

```bash
codesign --deep --force --options runtime \
  --sign "Developer ID Application: YOUR NAME (TEAMID)" "build/bin/App Cleaner.app"
ditto -c -k --keepParent "build/bin/App Cleaner.app" "App Cleaner.zip"
xcrun notarytool submit "App Cleaner.zip" --keychain-profile "notary" --wait
xcrun stapler staple "build/bin/App Cleaner.app"
```

## Credits

A GUI port of [mac-cleaner-cli](https://github.com/guhcostan/mac-cleaner-cli) by
[@guhcostan](https://github.com/guhcostan) — same categories, thresholds and safety
philosophy, rebuilt on a Go engine.

## License

[MIT](LICENSE)
