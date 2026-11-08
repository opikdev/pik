# Changelog

All notable changes to pik are recorded here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.3.0] - 2026-11-08

### Added

- `pik add`, which copies an add-on such as `agent-harness` into an existing project without overwriting files, turns on its Git hooks, and records it in `.pik.yaml`. It supports `--dry-run`, `--yes`, `--no-setup`, and `--json`.
- An `add` section in `pik.yaml`, for add-ons.
- `pik doctor` checks the tools an add-on needs, too.

## [0.2.0] - 2026-11-08

### Changed

- pik moved from opikdev/starters to its own repo, opikdev/pik. Install it with `go install github.com/opikdev/pik/cmd/pik@latest`, and use `https://raw.githubusercontent.com/opikdev/pik/main/install.sh` for the install script.
- Releases are published from opikdev/pik, so verify their signatures with that repo in the certificate identity.

### Fixed

- Catalog entries with a type this release doesn't know are skipped instead of failing every command, so future entry types won't break it.

## [0.1.1] - 2026-11-08

### Fixed

- `pik --version` reports the release when installed with `go install`, instead of `dev`.

### Changed

- `create-pik` is published with npm trusted publishing, so releases need no npm token.

## [0.1.0] - 2026-11-08

### Added

- `pik new`, `pik list`, and `pik doctor`, with prompts, `--set`, `--yes`, and `--json`.
- Releases for macOS, Linux, and Windows, through Homebrew, Scoop, an install script, `go install`, and `npm create pik`, with signed checksums.
