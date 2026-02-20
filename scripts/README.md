# Mockelot Build Scripts

Build logic lives here. Laminar CI jobs are thin dispatchers that `exec` these scripts.

## Build Scripts

| Script | Description | Usage |
|--------|-------------|-------|
| `_common.sh` | Shared library (version, logging, paths) | Sourced by all scripts |
| `build-linux.sh` | Linux amd64 binary via Docker | `./scripts/build-linux.sh` |
| `build-windows.sh` | Windows amd64 binary (cross-compile via Docker) | `./scripts/build-windows.sh` |
| `build-macos.sh` | macOS universal binary via SSH to macOS host | `CI_KEYCHAIN_PASSWORD=... ./scripts/build-macos.sh` |
| `build-appimage.sh` | AppImage for Debian 12 or 13 | `./scripts/build-appimage.sh debian12\|debian13` |
| `build-appimage-container.sh` | Runs inside Docker container (called by build-appimage.sh) | Not called directly |
| `build-deb.sh` | Convert AppImage to .deb package | `./scripts/build-deb.sh <version> <appimage-path>` |
| `build-all.sh` | Build all 5 platform artifacts | `./scripts/build-all.sh [--laminar]` |
| `release.sh` | Full release workflow (build + tag + GitHub release) | `./scripts/release.sh v0.3.6` |
| `clean.sh` | Remove all build artifacts | `./scripts/clean.sh` |

## Two Entry Points, Same Scripts

```
Developer runs directly:           Laminar dispatches:
  ./scripts/build-linux.sh           mockelot-linux-build.run → exec scripts/build-linux.sh
  ./scripts/build-windows.sh        mockelot-windows-build.run → exec scripts/build-windows.sh
  ./scripts/build-macos.sh          mockelot-macos-build.run → exec scripts/build-macos.sh
  ./scripts/build-appimage.sh d12   mockelot-appimage-d12.run → exec scripts/build-appimage.sh debian12
  ./scripts/build-appimage.sh d13   mockelot-appimage-d13.run → exec scripts/build-appimage.sh debian13
  ./scripts/build-all.sh            mockelot-build-all.run → exec scripts/build-all.sh --laminar
```

## Test Scripts

| Script | Description |
|--------|-------------|
| `test-socks5.sh` | Test SOCKS5 proxy functionality |

## Output

All build artifacts land in `dist/`:
```
dist/
├── linux/
│   ├── mockelot-linux-amd64.tar.gz
│   ├── mockelot-<ver>-debian12-x86_64.AppImage
│   ├── mockelot-<ver>-debian13-x86_64.AppImage
│   ├── mockelot_<ver>-debian12_amd64.deb
│   ├── mockelot_<ver>-debian13_amd64.deb
│   └── checksums.txt
├── windows/
│   └── mockelot-windows-amd64.zip
└── macos/
    └── mockelot-darwin-universal.zip
```
