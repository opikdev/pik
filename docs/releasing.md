# Release pik

A `vX.Y.Z` tag on this repo runs `.github/workflows/release.yml`, which publishes everywhere at once:

| Channel | What happens |
|---|---|
| GitHub release | Binaries for macOS, Linux, and Windows on amd64 and arm64, with SBOMs and `checksums.txt` signed by cosign |
| Homebrew | Updates the cask in [opikdev/homebrew-tap](https://github.com/opikdev/homebrew-tap) |
| Scoop | Updates the manifest in [opikdev/scoop-bucket](https://github.com/opikdev/scoop-bucket) |
| Go | The same tag is the Go module version, so `go install github.com/opikdev/pik/cmd/pik@latest` resolves it |
| npm | Publishes `create-pik` at the same version through trusted publishing, with provenance |

## Set up publishing once

- Save `PUBLISH_TOKEN` as a repo secret: a fine-grained GitHub token with read and write **Contents** on `homebrew-tap` and `scoop-bucket`.
- On npmjs.com, add a trusted publisher to `create-pik`: GitHub Actions, repository `opikdev/pik`, workflow `release.yml`, with the **npm publish** permission. npm then accepts this workflow without a token.

## Cut a release

1. Move the `Unreleased` entries in `CHANGELOG.md` under the new version, and merge that change through a pull request, because `main` only accepts pull requests.
2. Make sure CI is green on `main`, including the `release-config` job, which builds a full snapshot.
3. Tag the merge commit on `main` and push the tag:

   ```sh
   git switch main && git pull
   git tag -a v0.2.0 -m v0.2.0
   git push origin v0.2.0
   ```

4. Check that the release workflow passed, then run `brew upgrade pik` and `pik --version`.

## Verify a download

Releases up to v0.1.1 were published from opikdev/starters, so verify those with that repo in the identity instead.

```sh
cosign verify-blob \
  --certificate-identity 'https://github.com/opikdev/pik/.github/workflows/release.yml@refs/tags/v0.2.0' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --bundle checksums.txt.sigstore.json checksums.txt
sha256sum --check --ignore-missing checksums.txt
```
