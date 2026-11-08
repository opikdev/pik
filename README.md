# pik

A CLI that creates projects from agent-ready starter templates. It reads the [starters catalog](https://github.com/opikdev/starters), downloads the release you pick, and customizes it for your project.

[![CI](https://github.com/opikdev/pik/actions/workflows/ci.yml/badge.svg)](https://github.com/opikdev/pik/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Features

- **Released code only:** `pik new` downloads the tag the catalog pins, never a branch.
- **Prompts or one line:** answer questions, or pass `--set name=value --yes` from a script or agent.
- **No placeholders:** starters stay runnable, and `pik.yaml` declares the text to replace.
- **Consent first:** it shows a starter's setup commands and asks before running them.
- **Add-ons for existing repos:** `pik add agent-harness` adds files without overwriting yours.
- **Recorded answers:** `.pik.yaml` keeps the template, add-ons, versions, commits, and answers for later updates.
- **Agent-friendly:** every command takes `--json`.
- **Signed releases:** static binaries for macOS, Linux, and Windows, with checksums signed by cosign.

## Quick start

Install pik, then create a project:

```sh
brew install opikdev/tap/pik
pik new api-hono-starter my-api
```

Or run it without installing, with Node 20 or later:

```sh
npm create pik@latest
```

| Method | Command |
|---|---|
| Homebrew, on macOS and Linux | `brew install opikdev/tap/pik` |
| Scoop, on Windows | `scoop bucket add opikdev https://github.com/opikdev/scoop-bucket`, then `scoop install pik` |
| Install script, on macOS and Linux | `curl -fsSL https://raw.githubusercontent.com/opikdev/pik/main/install.sh \| sh` |
| npm, without installing | `npm create pik@latest` |
| Go 1.27 or later | `go install github.com/opikdev/pik/cmd/pik@latest` |

Binaries are on the [releases page](https://github.com/opikdev/pik/releases), with signed checksums. The install script always checks the checksum, and checks the signature too when `cosign` is installed. Releases up to v0.1.1 are on the [starters releases page](https://github.com/opikdev/starters/releases).

## Usage

| Command | Does |
|---|---|
| `pik list` | Lists the starters and add-ons in the catalog |
| `pik doctor [name]` | Checks that Git, and the tools a starter or add-on needs, are installed |
| `pik new [starter] [dir]` | Creates a project. With no arguments, it asks for everything |
| `pik add [add-on] [dir]` | Adds an add-on to an existing project, such as `agent-harness`. It keeps every file that already exists |

```sh
pik new api-hono-starter my-api --set owner=acme --yes
```

| Flag | Applies to | Does |
|---|---|---|
| `--set name=value` | `new` | Answers a starter prompt. Repeat it for each prompt |
| `--yes` | `new`, `add` | Accepts defaults and runs setup commands without asking |
| `--no-setup` | `new`, `add` | Skips the setup commands |
| `--dry-run` | `add` | Lists the files it would add and keep, without writing anything |
| `--no-git` | `new` | Skips `git init` |
| `--json` | All | Prints JSON, for agents and scripts |
| `--catalog <url-or-file>` | All | Reads another catalog |

| Variable | Default | Description |
|---|---|---|
| `PIK_CATALOG` | `catalog.yaml` on `main` in opikdev/starters | Catalog URL or file |
| `PIK_RAW_URL` | `https://raw.githubusercontent.com` | Where pik reads `pik.yaml` |
| `PIK_ARCHIVE_URL` | `https://codeload.github.com` | Where `pik new` downloads archives |

Starters describe themselves in `pik.yaml`. See the [pik.yaml reference](docs/pik-yaml.md).

To work on pik, you need Go 1.27, [golangci-lint](https://golangci-lint.run/) v2, and Node 20 for `create-pik`:

```sh
gofmt -l .
golangci-lint run
go test -race ./...
test/install.test.sh
(cd create-pik && npm test)
```

To publish a version, follow [Release pik](docs/releasing.md).

## Project structure

| Path | Purpose |
|---|---|
| `cmd/pik/` | The binary's entry point |
| `internal/` | Commands, catalog and manifest parsing, downloads, and project generation |
| `create-pik/` | The npm package behind `npm create pik` |
| `install.sh` | The install script for macOS and Linux, tested by `test/install.test.sh` |
| `.goreleaser.yaml` | Builds, signs, and publishes every release channel |
| `docs/` | [Architecture](docs/architecture.md), the [pik.yaml reference](docs/pik-yaml.md), and [decisions](docs/decisions/) |

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request, and follow the [code of conduct](CODE_OF_CONDUCT.md).

## Security

Report vulnerabilities privately, as described in [SECURITY.md](SECURITY.md).

## License

Released under the [MIT License](LICENSE).
