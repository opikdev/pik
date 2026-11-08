# pik

Create projects from the [starters](../../README.md) catalog.

## Install from source

You need Go 1.27 or later:

```sh
go install github.com/opikdev/starters/packages/cli@latest
```

## Use it

| Command | Does |
|---|---|
| `pik list` | Lists the starters and add-ons in the catalog |
| `pik doctor [starter]` | Checks that Git, and the tools a starter needs, are installed |
| `pik new [starter] [dir]` | Creates a project. With no arguments, it asks for everything |

```sh
pik new api-hono-starter my-api --set owner=acme --yes
```

| Flag | Applies to | Does |
|---|---|---|
| `--set name=value` | `new` | Answers a starter prompt. Repeat it for each prompt |
| `--yes` | `new` | Accepts defaults and runs the starter's setup commands without asking |
| `--no-setup` | `new` | Skips the setup commands |
| `--no-git` | `new` | Skips `git init` |
| `--json` | All | Prints JSON, for agents and scripts |
| `--catalog <url-or-file>` | All | Reads another catalog |

`pik new` downloads the starter at the tag pinned in the catalog, so a project never gets unreleased code. It shows the setup commands and asks before running them. It writes `.pik.yaml` with the template, version, commit, and answers, so `pik update` can merge later releases.

## Configure downloads

| Variable | Default | Description |
|---|---|---|
| `PIK_CATALOG` | The `catalog.yaml` on `main` in opikdev/starters | Catalog URL or file |
| `PIK_RAW_URL` | `https://raw.githubusercontent.com` | Where `pik doctor` reads `pik.yaml` |
| `PIK_ARCHIVE_URL` | `https://codeload.github.com` | Where `pik new` downloads archives |

## Work on pik

```sh
gofmt -l .
golangci-lint run
go test -race ./...
```

Starters describe themselves in `pik.yaml`. See the [pik.yaml reference](../../docs/pik-yaml.md).
