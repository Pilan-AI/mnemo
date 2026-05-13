# Releasing mnemo

## Cutting a release

1. Update `CHANGELOG.md` with the new version's notes.
2. Tag and push:

   ```bash
   git tag v1.x.y
   git push origin v1.x.y
   ```

3. The [`Release`](.github/workflows/release.yml) workflow fires on the tag push.
   It uses GoReleaser to create the GitHub Release, upload cross-platform binary
   archives + checksums, and bump `Pilan-AI/homebrew-tap`.
4. The workflow verifies that the GitHub Release has all expected assets and
   that the Homebrew formula points at the same tag.

That's it — `brew upgrade mnemo` picks up the new version as soon as the release
workflow completes.

## Required secrets

The release workflow needs one repo secret on `Pilan-AI/mnemo`:

- **`HOMEBREW_TAP_PAT`** — a fine-grained personal access token with
  `Contents: Read and write` on `Pilan-AI/homebrew-tap`. Set at
  **Settings → Secrets and variables → Actions → New repository secret**.

If the token expires, the workflow will fail with a 401 from the GitHub API —
rotate and re-save the secret.

## Manual fallback

If the workflow is broken, bump the tap by hand:

```bash
TAG=v1.x.y
SHA=$(curl -sL https://github.com/Pilan-AI/mnemo/archive/refs/tags/$TAG.tar.gz | shasum -a 256 | awk '{print $1}')
echo "url:    https://github.com/Pilan-AI/mnemo/archive/refs/tags/$TAG.tar.gz"
echo "sha256: $SHA"
```

Then update `Pilan-AI/homebrew-tap/Formula/mnemo.rb` with those two values and
open a PR.

## Why this matters

See `Pilan-AI/mnemo#8` — without the auto-bump workflow the tap drifted 7
releases behind upstream (v1.0.0 → v1.3.4) before anyone noticed. Don't let that
happen again.
