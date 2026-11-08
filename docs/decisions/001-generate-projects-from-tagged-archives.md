# 005. Generate projects from tagged archives

- **Status**: accepted
- **Context**: `pik new` must give every user the same released code, without needing Git, and must work for any stack. Copier-style template languages would put placeholders in starters, which would stop them from running as they are.
- **Decision**: Download the GitHub archive of the tag pinned in `catalog.yaml`, and customize it with plain text replacements declared in each starter's `pik.yaml`. Record the template, tag, commit, and answers in `.pik.yaml`.
- **Consequences**:
  - Starters stay runnable, and the archive carries no Git history.
  - The recorded commit and answers let `pik update` re-render both versions later.
  - Replacements fail loudly when the starter changes, and parent CI generates every starter to catch it.
  - Removing modules isn't supported yet, because it needs more than text replacement.
