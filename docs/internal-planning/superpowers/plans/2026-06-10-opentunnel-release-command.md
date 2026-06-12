# OpenTunnel Release Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a repeatable local release command that prepares `VERSION`, creates a published GitHub Release, and lets the existing release workflow publish GHCR images.

**Architecture:** Put all release logic in `scripts/release.sh`, with testable shell functions and a thin opencode `/release` command wrapper. The script performs preflight checks, infers or accepts a SemVer bump, updates `VERSION`, verifies, commits, pushes, creates the GitHub Release, then reopens development by setting `VERSION=dev`. Documentation explains both opencode and direct script usage.

**Tech Stack:** POSIX-ish Bash, Git, GitHub CLI (`gh`), opencode project command configuration, Go verification commands.

---

## File Structure

- Create: `scripts/release.sh` - release automation script with testable functions and a main entrypoint.
- Create: `scripts/release_test.sh` - shell tests for pure release calculation logic.
- Create: `.opencode/opencode.json` - project slash command configuration for `/release`.
- Modify: `docs/public-v1/operations.md` - document `/release` and `scripts/release.sh` usage.
- Add: `docs/superpowers/specs/2026-06-10-opentunnel-release-command-design.md` - approved design spec already written during brainstorming.

---

### Task 1: Add Testable Release Script Logic

**Files:**
- Create: `scripts/release.sh`
- Create: `scripts/release_test.sh`

- [ ] **Step 1: Create failing shell tests**

Create `scripts/release_test.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/release.sh"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

assert_eq() {
  local got="$1"
  local want="$2"
  local label="$3"
  if [ "$got" != "$want" ]; then
    fail "$label: got $got want $want"
  fi
}

assert_fail() {
  local label="$1"
  shift
  if "$@" >/tmp/opentunnel-release-test.out 2>/tmp/opentunnel-release-test.err; then
    fail "$label: command succeeded unexpectedly"
  fi
}

assert_eq "$(bump_version 0.1.0 patch)" "0.1.1" "patch bump"
assert_eq "$(bump_version 0.1.0 minor)" "0.2.0" "minor bump"
assert_eq "$(bump_version 0.1.0 major)" "1.0.0" "major bump"

assert_eq "$(detect_bump $'docs: update readme')" "patch" "fallback bump"
assert_eq "$(detect_bump $'fix: repair relay')" "patch" "fix bump"
assert_eq "$(detect_bump $'feat: add relay image')" "minor" "feat bump"
assert_eq "$(detect_bump $'fix!: change invite format')" "major" "bang bump"
assert_eq "$(detect_bump $'chore: release\n\nBREAKING CHANGE: changed protocol')" "major" "breaking body bump"

assert_eq "$(validate_bump '')" "auto" "empty bump defaults to auto"
assert_eq "$(validate_bump patch)" "patch" "patch override"
assert_eq "$(validate_bump minor)" "minor" "minor override"
assert_eq "$(validate_bump major)" "major" "major override"
assert_fail "invalid bump" validate_bump banana

assert_eq "$(latest_semver_tag_from_list $'0.1.0\n0.2.0\n1.0.0')" "1.0.0" "latest semver tag"
assert_eq "$(latest_semver_tag_from_list $'v1.0.0\n0.3.0\nnot-a-version')" "0.3.0" "ignore v-prefixed and invalid tags"
assert_eq "$(latest_semver_tag_from_list '')" "0.0.0" "no tags initializes from zero"

rm -f /tmp/opentunnel-release-test.out /tmp/opentunnel-release-test.err
printf 'release script tests passed\n'
```

- [ ] **Step 2: Run tests to verify failure**

Run: `bash scripts/release_test.sh`

Expected: FAIL because `scripts/release.sh` does not exist yet.

- [ ] **Step 3: Create release script with pure functions**

Create `scripts/release.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

die() {
  printf 'opentunnel release: %s\n' "$1" >&2
  exit 1
}

validate_bump() {
  local bump="${1:-}"
  case "$bump" in
    "") printf 'auto\n' ;;
    patch|minor|major) printf '%s\n' "$bump" ;;
    *) die "bump must be patch, minor, major, or empty" ;;
  esac
}

bump_version() {
  local version="$1"
  local bump="$2"
  local major minor patch
  IFS=. read -r major minor patch <<< "$version"
  case "$bump" in
    patch) patch=$((patch + 1)) ;;
    minor) minor=$((minor + 1)); patch=0 ;;
    major) major=$((major + 1)); minor=0; patch=0 ;;
    *) die "unsupported bump $bump" ;;
  esac
  printf '%s.%s.%s\n' "$major" "$minor" "$patch"
}

detect_bump() {
  local log_text="$1"
  if printf '%s\n' "$log_text" | grep -Eq '(^|\n)BREAKING CHANGE:'; then
    printf 'major\n'
    return
  fi
  if printf '%s\n' "$log_text" | grep -Eq '^[a-zA-Z]+(\([^)]+\))?!:'; then
    printf 'major\n'
    return
  fi
  if printf '%s\n' "$log_text" | grep -Eq '^feat(\([^)]+\))?:'; then
    printf 'minor\n'
    return
  fi
  if printf '%s\n' "$log_text" | grep -Eq '^fix(\([^)]+\))?:'; then
    printf 'patch\n'
    return
  fi
  printf 'patch\n'
}

latest_semver_tag_from_list() {
  local tags="$1"
  local latest
  latest="$(printf '%s\n' "$tags" | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' | sort -t. -k1,1n -k2,2n -k3,3n | tail -n 1 || true)"
  if [ -z "$latest" ]; then
    printf '0.0.0\n'
    return
  fi
  printf '%s\n' "$latest"
}

latest_semver_tag() {
  latest_semver_tag_from_list "$(git tag --list)"
}

commit_log_since() {
  local latest_tag="$1"
  if [ "$latest_tag" = "0.0.0" ]; then
    git log --format='%s%n%b'
    return
  fi
  git log "${latest_tag}..HEAD" --format='%s%n%b'
}

ensure_clean_main() {
  local branch
  branch="$(git branch --show-current)"
  [ "$branch" = "main" ] || die "must run from main branch"
  git fetch origin main --tags
  [ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] || die "local main must match origin/main"
  [ -z "$(git status --porcelain)" ] || die "working tree must be clean, including untracked files"
}

ensure_release_tools() {
  command -v gh >/dev/null 2>&1 || die "gh is required"
  gh auth status >/dev/null 2>&1 || die "gh auth status failed"
}

ensure_dev_version() {
  local version
  version="$(tr -d '[:space:]' < VERSION)"
  [ "$version" = "dev" ] || die "VERSION must be dev before preparing a release"
}

run_verification() {
  go test ./... -count=1
  go vet ./...
  go mod tidy -diff
  go test -race ./... -count=1
  go build ./cmd/opentunnel
  rm -f ./opentunnel
}

main() {
  local requested_bump bump latest_tag log_text next_version
  requested_bump="$(validate_bump "${1:-}")"
  ensure_clean_main
  ensure_release_tools
  ensure_dev_version

  latest_tag="$(latest_semver_tag)"
  if [ "$requested_bump" = "auto" ]; then
    log_text="$(commit_log_since "$latest_tag")"
    bump="$(detect_bump "$log_text")"
  else
    bump="$requested_bump"
  fi
  next_version="$(bump_version "$latest_tag" "$bump")"

  printf '%s\n' "$next_version" > VERSION
  run_verification
  git add VERSION
  git commit -m "chore: release $next_version"
  release_commit="$(git rev-parse HEAD)"
  git tag "$next_version" "$release_commit"
  git push origin "$next_version"
  gh release create "$next_version" --verify-tag --generate-notes --latest
  printf 'dev\n' > VERSION
  git add VERSION
  git commit -m "chore: reopen development"
  git push origin main
  printf 'released %s\n' "$next_version"
}

if [[ "${BASH_SOURCE[0]}" = "$0" ]]; then
  main "${1:-}"
fi
```

- [ ] **Step 4: Make scripts executable**

Run: `chmod +x scripts/release.sh scripts/release_test.sh`

Expected: command exits 0.

- [ ] **Step 5: Run tests to verify pass**

Run: `bash scripts/release_test.sh`

Expected: prints `release script tests passed`.

- [ ] **Step 6: Commit scripts**

```bash
git add scripts/release.sh scripts/release_test.sh
git commit -m "chore: add release helper script"
```

---

### Task 2: Add Opencode Slash Command

**Files:**
- Create: `.opencode/opencode.json`

- [ ] **Step 1: Create project opencode config**

Create `.opencode/opencode.json`:

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

- [ ] **Step 2: Validate JSON syntax**

Run: `go test ./... -count=1`

Expected: PASS. This does not validate opencode config, but confirms repository tests are unaffected.

- [ ] **Step 3: Validate config contains schema and command**

Run: `grep -n '"\$schema"\|"release"\|"template"' .opencode/opencode.json`

Expected: prints matching schema, release, and template lines.

- [ ] **Step 4: Commit slash command config**

```bash
git add .opencode/opencode.json
git commit -m "chore: add release slash command"
```

---

### Task 3: Document Release Command

**Files:**
- Modify: `docs/public-v1/operations.md`

- [ ] **Step 1: Update release docs**

In `docs/public-v1/operations.md`, update the manual release process to include the command-first flow before the existing manual details:

```markdown
## Release Command

From a clean `main` checkout, use opencode:

```text
/release
```

To force a bump:

```text
/release patch
/release minor
/release major
```

The command runs `scripts/release.sh`, which verifies the working tree and `origin/main` state, infers or applies the SemVer bump, updates `VERSION`, runs verification, commits the release, pushes the release tag, creates a published GitHub Release, then commits `VERSION=dev` back to `main`.

The same flow is available without opencode:

```bash
scripts/release.sh
scripts/release.sh patch
scripts/release.sh minor
scripts/release.sh major
```

Restart opencode after pulling changes that add `.opencode/opencode.json`; running sessions do not reload slash commands.
```

Keep the manual release process as a fallback for debugging or recovery if the script fails.

- [ ] **Step 2: Verify docs references**

Run: `grep -n '/release\|scripts/release.sh\|Restart opencode' docs/public-v1/operations.md`

Expected: prints references to `/release`, `scripts/release.sh`, and restart guidance.

- [ ] **Step 3: Commit docs**

```bash
git add docs/public-v1/operations.md
git commit -m "docs: document release command"
```

---

### Task 4: Final Verification

**Files:**
- `scripts/release.sh`
- `scripts/release_test.sh`
- `.opencode/opencode.json`
- `docs/public-v1/operations.md`
- `docs/superpowers/specs/2026-06-10-opentunnel-release-command-design.md`
- `docs/superpowers/plans/2026-06-10-opentunnel-release-command.md`

- [ ] **Step 1: Run release script tests**

Run: `bash scripts/release_test.sh`

Expected: prints `release script tests passed`.

- [ ] **Step 2: Run Go tests**

Run: `go test ./... -count=1`

Expected: PASS.

- [ ] **Step 3: Run Go vet**

Run: `go vet ./...`

Expected: PASS with no output.

- [ ] **Step 4: Run module tidy check**

Run: `go mod tidy -diff`

Expected: PASS with no diff output.

- [ ] **Step 5: Run race tests**

Run: `go test -race ./... -count=1`

Expected: PASS.

- [ ] **Step 6: Verify release command files**

Run: `grep -n 'command\|release\|template' .opencode/opencode.json && grep -n 'gh release create\|chore: release\|chore: reopen development' scripts/release.sh`

Expected: prints opencode command config and release script release-creation lines.

- [ ] **Step 7: Check status**

Run: `git status --short`

Expected: only intended files are modified or staged. Unrelated `.agents/` or `skills-lock.json` should not be included unless the user explicitly asks.
