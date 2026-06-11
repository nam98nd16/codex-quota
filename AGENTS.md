# Release Workflow

## Scope

- Use this repository-local guide when publishing a release.
- Keep the workflow minimal and deterministic.

## Branch Policy

- Use `fork/main` as the canonical branch for active work and releases.
- Local `main` should track `fork/main` before starting new work.
- Treat `origin/main` as the upstream project reference only; do not use it as the release base.
- Keep any local upstream-tracking branch clearly named, for example `upstream-main`.
- Do not use `fork/release-pr3` for normal work or releases unless explicitly requested.
- Do not delete legacy branches such as `release-pr3` unless explicitly requested.

## Publish Steps

1. Check `git status --short --branch`, `git diff --stat`, and recent commits before releasing.
2. Commit only the intended changes.
3. Create the next semver tag, for example `v0.3.23` after `v0.3.22`.
4. Push the branch to `fork/main`.
5. Push the release tag to `fork`.
6. Publish with GoReleaser:
   - `GITHUB_TOKEN="$(gh auth token)" HOMEBREW_TAP_GITHUB_TOKEN="$(gh auth token)" goreleaser release --clean`
7. Reinstall the Homebrew formula:
   - `brew update && brew reinstall nam98nd16/tap/codex-quota`
8. Verify the installed version:
   - `cq --version`

## Important Notes

- Use the `fork` remote for release pushes, not `origin`.
- Do not rely on GitHub Actions manual dispatch; this repo's release workflow is tag-triggered.
- After publishing, confirm the release tag exists on GitHub and the Homebrew formula was updated.
- If the release is only a doc/test/config change, still follow the same release flow if a versioned binary should be published.

## Versioning

- Increment only the patch version for routine fixes and UX tweaks unless a larger change explicitly warrants otherwise.
- Keep release commit messages short and conventional, such as `feat:` or `fix:`.
