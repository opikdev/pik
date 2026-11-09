# pik.yaml reference

Every starter and add-on keeps a `pik.yaml` at its root. `pik new` reads it from the tagged archive, then removes it from the generated project. `pik add` reads an add-on's manifest the same way.

```yaml
schema: 1
requires:
  - name: node
    version: 26.0.0
prompts:
  - name: name
    message: Project name, in kebab-case
    pattern: ^[a-z][a-z0-9-]{0,49}$
    default: "{{dir}}"
replace:
  - from: api-hono-starter
    to: "{{name}}"
    files: [package.json]
exclude: [.baseline.sha256]
setup: [pnpm install]
next: [docker compose up --build --wait api]
check: [pnpm verify]
```

| Field | Type | Description |
|---|---|---|
| `schema` | number | Always `1` |
| `requires` | list | Tools `pik doctor` checks. `name` is the command, `version` an optional minimum, and `versionArgs` overrides `--version` |
| `prompts` | list | Values to ask for. Each has `name`, `message`, and optionally `pattern`, `default`, and `optional`. `{{dir}}` is the project folder name. A prompt named `owner` with no `default` defaults to your GitHub username, from `git config github.user` or the GitHub CLI, when it matches `pattern` |
| `replace` | list | Text to swap in listed files, in order. See [Replacement order](#replacement-order). `to` can use `{{prompt}}` values. A replacement is skipped when one of its values is empty |
| `exclude` | list | Paths to delete from the generated project |
| `setup` | list | Shell commands to run after generating, with consent |
| `next` | list | Commands to suggest when pik finishes |
| `check` | list | Commands that prove a generated project works. Parent CI runs them |

Unknown fields are rejected. A replacement whose `from` text is missing from a listed file fails the generation, so a starter and its manifest can't drift apart silently.

## Replacement order

pik runs `replace` rules in the order you list them, and each rule works on the file as the earlier rules left it:

- A rule's `from` must still be in the file after the earlier rules run, or `pik new` fails.
- A later rule also rewrites text that an earlier rule wrote.
- A skipped rule leaves its text for the later rules.

So list the longest, most specific `from` first, and make sure no `to` value can contain a later rule's `from`:

```yaml
replace:
  - from: opikdev/api-hono-starter
    to: "{{owner}}/{{name}}"
    files: [README.md]
  - from: api-hono-starter
    to: "{{name}}"
    files: [README.md]
```

| Answers | `README.md` before | After rule 1 | After rule 2 |
|---|---|---|---|
| `name=my-api`, `owner=acme` | `# api-hono-starter`, `opikdev/api-hono-starter` | `# api-hono-starter`, `acme/my-api` | `# my-api`, `acme/my-api` |
| `name=my-api`, no owner | `# api-hono-starter`, `opikdev/api-hono-starter` | Skipped | `# my-api`, `opikdev/my-api` |
| `name=api-hono-starter-v2`, `owner=acme` | `# api-hono-starter`, `opikdev/api-hono-starter` | `# api-hono-starter`, `acme/api-hono-starter-v2` | `# api-hono-starter-v2`, `acme/api-hono-starter-v2-v2` |

In the last row, rule 2 renames the text rule 1 wrote. If the two rules were swapped, the `api-hono-starter` rule would run first and turn the link into `opikdev/my-api`, so the `opikdev/api-hono-starter` rule wouldn't find its text and `pik new` would fail.

## Add-ons

An add-on uses `add` instead of `prompts`, `replace`, and `exclude`:

```yaml
schema: 1
requires:
  - name: git
  - name: bash
add:
  from: template
setup: [bash .agents/check.sh]
check: [bash .agents/check.sh]
```

| Field | Type | Description |
|---|---|---|
| `add.from` | string | Folder in the add-on whose files `pik add` copies into the project |

`pik add` copies every file in `add.from` that doesn't exist in the project yet, and keeps the ones that do. If the add-on adds `.githooks` to a Git repo root with no `core.hooksPath`, pik turns the hooks on. It records the add-on under `addons` in `.pik.yaml`, then runs `setup` with consent. If `.pik.yaml` already lists the add-on, `pik add` stops without changing anything. Parent CI adds each released add-on to an empty repo and runs `check` there.
