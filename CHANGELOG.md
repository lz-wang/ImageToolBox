# Changelog

All notable changes to this project will be documented in this file.

## [v0.2.0] - 2026-04-22

### Added

- Added `crop` command with anchor-based percentage cropping for `left`, `right`, `top`, `bottom`, corners, and `center`.
- Added Windows release artifacts for both `amd64` and `arm64`.
- Added Pushover notifications for CI and release workflow completion.

### Changed

- Extended build and release workflows to produce bundled binaries for macOS, Linux, and Windows in parallel.
- Updated GitHub Actions dependencies to current major versions and enabled Node.js 24 preflight mode.
- Improved Windows runner dependency setup to prefer preinstalled `cmake` and `nasm`, avoiding ARM64 Chocolatey install failures.

## [v0.1.0] - 2026-04-07

- Initial tagged release.
