# Release pik

A `vX.Y.Z` tag on this repo runs `.github/workflows/release.yml`, which publishes everywhere at once:

| Channel | What happens |
|---|---|
| GitHub release | Binaries for macOS, Linux, and Windows on amd64 and arm64, with SBOMs and `checksums.txt` signed by cosign |
| Homebrew | Updates the cask in [opikdev/homebrew-tap](https://github.com/opikdev/homebrew-tap) |
| Scoop | Updates the manifest in [opikdev/scoop-bucket](https://github.com/opikdev/scoop-bucket) |
| Go | Adds a `packages/cli/vX.Y.Z` tag, so `go install` resolves the version |
| npm | Publishes `create-pik` at the same version, with provenance |

## Set up the secrets once

| Secret | Holds |
|---|---|
| `PUBLISH_TOKEN` | A fine-grained GitHub token with read and write **Contents** on `homebrew-tap` and `scoop-bucket` |
| `NPM_TOKEN` | An npm granular access token that can publish `create-pik` |

## Cut a release

1. Move the `Unreleased` entries in `CHANGELOG.md` under the new version.
2. Make sure CI is green on `main`, including the `release-config` job, which builds a full snapshot.
3. Tag and push:

   ```sh
   git tag -a v0.2.0 -m v0.2.0
   git push origin v0.2.0
   ```

4. Check that the release workflow passed, then run `brew upgrade pik` and `pik --version`.

## Verify a download

```sh
cosign verify-blob \
  --certificate-identity 'https://github.com/opikdev/starters/.github/workflows/release.yml@refs/tags/v0.1.0' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --bundle checksums.txt.sigstore.json checksums.txt
sha256sum --check --ignore-missing checksums.txt
```
