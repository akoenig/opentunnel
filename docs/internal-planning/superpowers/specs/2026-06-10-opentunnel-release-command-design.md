# OpenTunnel Release Command Design

## Purpose

OpenTunnel releases should be repeatable from one local command. The command should prepare the repository state that the GitHub Release workflow expects, create a published GitHub Release, and let the existing GHCR workflow publish the container image.

## Goals

- Add a project slash command for opencode, `/release`.
- Put deterministic release logic in a repo script that also works without opencode.
- Infer the next SemVer version from conventional commit messages by default.
- Allow explicit `patch`, `minor`, or `major` override.
- Update `VERSION`, commit the release, push the release tag, and create a published GitHub Release with `gh` before reopening `main` for development.
- Reset `VERSION` back to `dev` on `main` after the release tag is created.

## Non-Goals

- Replacing the GitHub Release workflow that publishes GHCR images.
- Publishing images locally from the release script.
- Supporting `v`-prefixed versions or tags.
- Supporting prereleases in this milestone.
- Adding changelog generation beyond GitHub's generated release notes.

## Files

- `scripts/release.sh`: release automation script.
- `.opencode/opencode.json`: project slash command configuration.
- `docs/public-v1/operations.md`: release process documentation.

## Command UX

Default automatic bump:

```text
/release
```

Explicit bump override:

```text
/release patch
/release minor
/release major
```

The project opencode command should be configured in `.opencode/opencode.json` using the current schema's `command.release.template` field. The command should run the script with no argument or exactly one explicit bump argument:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "command": {
    "release": {
      "description": "Prepare and publish an OpenTunnel release.",
      "template": "If no argument was supplied, run scripts/release.sh. If the argument is exactly patch, run scripts/release.sh patch. If the argument is exactly minor, run scripts/release.sh minor. If the argument is exactly major, run scripts/release.sh major. For any other argument, tell the user the valid forms are: /release, /release patch, /release minor, or /release major, and do not run scripts/release.sh. Do not bypass script preflight checks."
    }
  }
}
```

The script remains the source of truth. The slash command should not duplicate release logic in prose beyond invoking the script with an optional bump argument.

After adding the opencode project command, users must restart opencode before `/release` appears in the running session.

## Script Preflight

The script should fail before changing files unless all preflight checks pass:

- Current branch is `main`.
- Working tree is clean.
- `origin/main` is fetched and local `main` equals `origin/main`.
- `VERSION` currently contains `dev`.
- `gh` is installed and authenticated.
- The requested bump override is empty, `patch`, `minor`, or `major`.
- A latest SemVer release tag exists, or the script can initialize from `0.0.0` if no release tags exist.

The clean working tree check intentionally includes untracked files. A release should start from a fully clean checkout.

## Version Detection

The script finds the latest SemVer tag without a leading `v`, such as `0.1.0`.

For automatic bump detection, inspect commit subjects and bodies since that tag:

- `BREAKING CHANGE:` in a commit body or footer -> `major`.
- Conventional commit subject with `!`, such as `feat!:` or `fix(scope)!:` -> `major`.
- Conventional commit subject beginning with `feat:` or `feat(scope):` -> `minor`.
- Conventional commit subject beginning with `fix:` or `fix(scope):` -> `patch`.
- If commits exist but none match those categories, default to `patch`.

Precedence is `major` over `minor` over `patch`.

Version bump examples:

- `0.1.0` + patch -> `0.1.1`
- `0.1.0` + minor -> `0.2.0`
- `0.1.0` + major -> `1.0.0`

## Release Flow

After preflight and version calculation, the script should:

1. Write the new version to `VERSION`.
2. Run the standard verification commands:
   - `go test ./... -count=1`
   - `go vet ./...`
   - `go mod tidy -diff`
   - `go test -race ./... -count=1`
   - `go build ./cmd/opentunnel`
   - `rm -f ./opentunnel`
3. Commit `VERSION` with message `chore: release X.Y.Z`.
4. Create a local tag `X.Y.Z` at the release commit.
5. Push only tag `X.Y.Z`.
6. Create a published GitHub Release with `gh release create X.Y.Z --verify-tag --generate-notes --latest`.
7. Write `dev` back to `VERSION`.
8. Commit `VERSION` with message `chore: reopen development`.
9. Push `main`.

The GitHub Release tag points at the release commit containing `VERSION=X.Y.Z`. The follow-up development commit returns `main` to `VERSION=dev`.

## Failure Handling

The script should use `set -euo pipefail` and stop at the first failing command.

If verification fails after writing `VERSION`, the script should restore `VERSION=dev` and stop. It should not create a release commit, tag, GitHub Release, or push anything.

If release creation fails after pushing the release tag, the script should best-effort delete the remote and local tag, avoid pushing `main`, and report the failure. The local release commit remains for inspection or manual recovery.

## Documentation

Update operations docs to describe:

- `/release` prepares and publishes a stable release.
- `scripts/release.sh` is available for non-opencode use.
- The command creates a release commit, a GitHub Release, and a follow-up `VERSION=dev` commit.
- The existing release workflow publishes GHCR images from the GitHub Release.

## Testing And Verification

Implementation should include shell-friendly tests or dry-run checks for pure logic where practical:

- Latest SemVer tag detection.
- Automatic bump detection for breaking, feature, fix, and fallback commits.
- Explicit bump override validation.
- Version bump arithmetic.

Full end-to-end release creation should not run in normal tests because it mutates the remote repository. The script should be reviewed and tested with local helper functions where possible.

## Acceptance Criteria

The feature is complete when:

- `/release` invokes the repo release script.
- `scripts/release.sh` can compute the next version from conventional commits.
- The script updates `VERSION`, commits, pushes, and creates a published GitHub Release.
- The script resets `VERSION` to `dev` after creating the release.
- Operations docs explain both `/release` and direct script usage.
