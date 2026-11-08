# 006. Ship pik as signed static binaries

- **Status**: accepted
- **Context**: `pik` must install in one step on macOS, Linux, and Windows, including for people without Go. A downloaded binary is a supply-chain risk unless users can check where it came from.
- **Decision**: Build static binaries with GoReleaser, sign `checksums.txt` with keyless cosign through GitHub OIDC, and publish to Homebrew, Scoop, an install script, and `create-pik` on npm.
- **Consequences**:
  - No channel needs a runtime, except `npm create pik`, which needs Node to fetch the binary.
  - The install script and `create-pik` always verify the checksum, and the install script verifies the signature when cosign is installed.
  - One tag publishes every channel, so tagging needs care. `docs/releasing.md` covers the steps.
  - macOS binaries aren't notarized, so the Homebrew cask clears the quarantine flag.
