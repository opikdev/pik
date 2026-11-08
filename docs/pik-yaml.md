# pik.yaml reference

Every starter keeps a `pik.yaml` at its root. `pik new` reads it from the tagged archive, then removes it from the generated project.

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
| `prompts` | list | Values to ask for. Each has `name`, `message`, and optionally `pattern`, `default`, and `optional`. `{{dir}}` is the project folder name |
| `replace` | list | Text to swap in listed files. `to` can use `{{prompt}}` values. A replacement is skipped when one of its values is empty |
| `exclude` | list | Paths to delete from the generated project |
| `setup` | list | Shell commands to run after generating, with consent |
| `next` | list | Commands to suggest when pik finishes |
| `check` | list | Commands that prove a generated project works. Parent CI runs them |

Unknown fields are rejected. A replacement whose `from` text is missing from a listed file fails the generation, so a starter and its manifest can't drift apart silently.
