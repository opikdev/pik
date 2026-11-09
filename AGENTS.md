# Agent guide

`pik`, the Go CLI that creates projects from the starters catalog in [opikdev/starters](https://github.com/opikdev/starters), plus its npm wrapper and install script.

| Path | Purpose |
|---|---|
| [docs/architecture.md](docs/architecture.md) | How pik reads the catalog and generates a project |
| [docs/pik-yaml.md](docs/pik-yaml.md) | The manifest format each starter declares |
| [docs/releasing.md](docs/releasing.md) | How a tag publishes every channel |
| [docs/decisions/](docs/decisions/) | Why it's built this way |
| `.agents/skills/` | Step-by-step guides for recurring tasks |
| `.mcp.json` | MCP servers for this project |

## Commands

| Task | Command |
|---|---|
| Format, lint, and test | `gofmt -l .`, `golangci-lint run`, `go test -race ./...` |
| Test the install script | `test/install.test.sh` |
| Test create-pik | `cd create-pik && npm test` |
| Build a release snapshot | `goreleaser release --snapshot --clean --skip=sign,publish` |
| Try a command | `go run ./cmd/pik list` |

## Rules

- Run every command above, and `.agents/check.sh`, before calling work done. The pre-commit hook runs only `.agents/check.sh`, and CI runs all of them.
- If the check blocks you, fix the cause. Never edit `.agents/check.sh`, `.githooks/`, or the harness workflow to get past it, and never use `--no-verify`. If the request can't be done without that, stop and ask.
- After cloning, run `git config core.hooksPath .githooks` once to turn on the hook.
- Never tag without the user's go-ahead, because a tag publishes to GitHub, Homebrew, Scoop, Go, and npm.
- Keep pik compatible with the live catalog. A new catalog field or entry type must be optional, because installed releases read the same file.
- When the `pik.yaml` format changes, update `docs/pik-yaml.md` in the same change.
- Don't edit files listed in `.baseline.sha256`. They come from [opikdev/starters](https://github.com/opikdev/starters).
- Keep this file a short map, and put detail in `docs/`.
- Fix any doc your change makes wrong, in the same change.
- Record significant decisions with the `record-decision` skill.
- Back important rules with a lint rule, hook, or CI check.
