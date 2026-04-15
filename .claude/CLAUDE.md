# mnemo — notes for Claude Code sessions

## Release checklist (CRITICAL — do not skip)

When the user asks to "cut a release", "tag a version", or merges a version bump:

1. Ensure `CHANGELOG.md` has an entry for the new version.
2. Tag: `git tag vX.Y.Z && git push origin vX.Y.Z`.
3. **Verify the `Bump Homebrew Formula` workflow ran.** It lives in
   `.github/workflows/release.yml` and requires the `HOMEBREW_TAP_PAT` repo
   secret. It opens a PR on `Pilan-AI/homebrew-tap`.
4. Review and merge the tap PR. Until it's merged, `brew upgrade mnemo` stays
   on the old version.
5. Optionally: `gh release create vX.Y.Z --generate-notes`.

If the workflow didn't fire or failed, fall back to the manual instructions in
[`RELEASING.md`](../RELEASING.md).

**History of this bug:** `Pilan-AI/mnemo#8` — the tap drifted from v1.0.0 to
v1.3.4 (7 releases) because releases were tagged without bumping the formula.
Shipping a tag is not shipping a release; the formula bump is the last mile.

## Other context

- Module path: `github.com/0xRaghu/mnemo` (note the `0xRaghu`, not `Pilan-AI`).
  The `go build -ldflags -X ...cmd.Version=` injection in the formula depends
  on this exact path.
- Homebrew tap: `Pilan-AI/homebrew-tap`, formula at `Formula/mnemo.rb`.
- License is AGPL-3.0-or-later (see `LICENSE`), not MIT — `CONTRIBUTING.md` has
  a stale MIT mention that should get fixed opportunistically.
