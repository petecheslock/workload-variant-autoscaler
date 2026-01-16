# Release Guide

This guide describes the process for creating releases of Workload-Variant-Autoscaler (WVA).

## Table of Contents

- [Release Process Overview](#release-process-overview)
- [Prerequisites](#prerequisites)
- [Version Numbering](#version-numbering)
- [Pre-Release Checklist](#pre-release-checklist)
- [Creating a Release](#creating-a-release)
- [Post-Release Tasks](#post-release-tasks)
- [Hotfix Releases](#hotfix-releases)
- [Rollback Procedure](#rollback-procedure)

## Release Process Overview

WVA follows semantic versioning and uses GitHub releases with automated CI/CD pipelines for building and publishing artifacts.

**Release Artifacts:**
- Docker images (controller, webhook)
- Helm charts
- CRD manifests
- Documentation

**Release Cadence:**
- Major releases: As needed for breaking changes
- Minor releases: Monthly or as features are ready
- Patch releases: As needed for critical bug fixes

## Prerequisites

**Required Access:**
- Write access to the GitHub repository
- Access to container registry (for publishing images)
- Access to Helm chart repository (if applicable)

**Required Tools:**
- Git
- GitHub CLI (`gh`)
- Docker or Podman
- Helm 3.x
- GPG key for signing tags (recommended)

## Version Numbering

WVA follows [Semantic Versioning 2.0.0](https://semver.org/):

- **Major version** (X.0.0): Breaking changes to API or behavior
- **Minor version** (0.X.0): New features, backwards compatible
- **Patch version** (0.0.X): Bug fixes, backwards compatible

**Examples:**
- `v0.5.0` - Minor release with new features
- `v0.5.1` - Patch release with bug fixes
- `v1.0.0` - Major release with breaking changes

**Pre-release versions:**
- `v0.6.0-alpha.1` - Alpha pre-release
- `v0.6.0-beta.1` - Beta pre-release
- `v0.6.0-rc.1` - Release candidate

## Pre-Release Checklist

Before creating a release, ensure:

### 1. Code Quality

- [ ] All CI checks pass on main branch
- [ ] Test coverage is acceptable
- [ ] No critical bugs or security issues
- [ ] Code review completed for all changes

```bash
# Run full test suite
make test

# Run e2e tests
make test-e2e

# Check for known vulnerabilities
make security-scan
```

### 2. Documentation

- [ ] CHANGELOG updated with release notes
- [ ] API documentation reflects changes
- [ ] User guide updated for new features
- [ ] Migration guide written (for breaking changes)
- [ ] All documentation links work

```bash
# Check for broken links
make docs-check-links
```

### 3. Dependencies

- [ ] Go dependencies updated to latest stable versions
- [ ] Kubernetes dependencies compatible with supported versions
- [ ] Security advisories reviewed for dependencies

```bash
# Update dependencies
go get -u ./...
go mod tidy

# Check for vulnerabilities
go list -json -m all | nancy sleuth
```

### 4. Version Updates

- [ ] Version number updated in all relevant files:
  - `charts/workload-variant-autoscaler/Chart.yaml` (version, appVersion)
  - `config/manager/kustomization.yaml` (image tag)
  - `Makefile` (IMG variable default)

## Creating a Release

### Step 1: Prepare Release Branch

```bash
# Ensure you're on main and up to date
git checkout main
git pull upstream main

# Create release branch
export VERSION="v0.6.0"
git checkout -b release-${VERSION}
```

### Step 2: Update Version Numbers

```bash
# Update Chart.yaml
vim charts/workload-variant-autoscaler/Chart.yaml
# Update:
#   version: 0.6.0
#   appVersion: v0.6.0

# Update default image tag in Makefile
vim Makefile
# Set IMG to new version

# Commit changes
git add charts/workload-variant-autoscaler/Chart.yaml Makefile
git commit -m "chore: bump version to ${VERSION}"
```

### Step 3: Update CHANGELOG

Create or update the CHANGELOG:

```bash
# Create changelog file for this version
vim docs/CHANGELOG-${VERSION}.md
```

Include:
- New features
- Bug fixes
- Breaking changes
- Deprecations
- Upgrade notes

Example structure:

```markdown
# Release v0.6.0

## Release Date
YYYY-MM-DD

## New Features
- Feature 1: Description (#PR)
- Feature 2: Description (#PR)

## Bug Fixes
- Fix 1: Description (#PR)
- Fix 2: Description (#PR)

## Breaking Changes
- Change 1: Migration instructions
- Change 2: Migration instructions

## Deprecations
- Deprecated feature: Removal timeline

## Upgrade Notes
Special considerations for upgrading from v0.5.x
```

Commit the changelog:

```bash
git add docs/CHANGELOG-${VERSION}.md
git commit -m "docs: add changelog for ${VERSION}"
```

### Step 4: Create Pull Request

```bash
# Push release branch
git push origin release-${VERSION}

# Create PR
gh pr create \
  --title "Release ${VERSION}" \
  --body "Release preparation for ${VERSION}. See docs/CHANGELOG-${VERSION}.md for details." \
  --base main
```

Wait for CI checks and get approval from maintainers.

### Step 5: Merge and Tag

```bash
# After PR is approved and merged, checkout main
git checkout main
git pull upstream main

# Create and push signed tag
git tag -s ${VERSION} -m "Release ${VERSION}"
git push upstream ${VERSION}
```

### Step 6: Trigger Release Workflow

The GitHub Actions release workflow should trigger automatically on tag push.

Monitor the workflow:

```bash
# Watch release workflow
gh run watch
```

The workflow will:
1. Build and push Docker images
2. Package and publish Helm charts
3. Create GitHub release with artifacts
4. Update documentation

### Step 7: Create GitHub Release

If not automated, create GitHub release manually:

```bash
gh release create ${VERSION} \
  --title "Release ${VERSION}" \
  --notes-file docs/CHANGELOG-${VERSION}.md \
  --draft=false \
  --prerelease=false
```

Or use the GitHub web UI:
1. Go to Releases page
2. Click "Draft a new release"
3. Select the tag
4. Fill in release notes (use CHANGELOG content)
5. Publish release

## Post-Release Tasks

### 1. Verify Release Artifacts

```bash
# Check Docker images are available
docker pull ghcr.io/llm-d-incubation/workload-variant-autoscaler:${VERSION}

# Check Helm chart is available
helm repo add wva <helm-repo-url>
helm repo update
helm search repo wva --version ${VERSION}
```

### 2. Update Documentation

- [ ] Update main README if needed
- [ ] Update installation guides with new version
- [ ] Announce release in community channels
- [ ] Update project status/roadmap

### 3. Create Next Milestone

```bash
# Create next milestone in GitHub
gh issue milestone create "v0.7.0" --due "YYYY-MM-DD"
```

### 4. Announce Release

- Post to community Slack channel
- Update project website (if applicable)
- Send email to mailing list (if applicable)
- Social media announcement (if applicable)

## Hotfix Releases

For critical bug fixes that can't wait for the next regular release:

### Step 1: Create Hotfix Branch

```bash
# Create hotfix branch from the release tag
export HOTFIX_VERSION="v0.6.1"
git checkout -b hotfix-${HOTFIX_VERSION} ${VERSION}
```

### Step 2: Apply Fix

```bash
# Cherry-pick fix commits or make changes directly
git cherry-pick <commit-hash>

# Or make changes and commit
git add .
git commit -m "fix: critical bug description"
```

### Step 3: Update Version and Changelog

```bash
# Update version numbers (see Step 2 in Creating a Release)
# Create changelog for hotfix
vim docs/CHANGELOG-${HOTFIX_VERSION}.md
git add docs/CHANGELOG-${HOTFIX_VERSION}.md
git commit -m "docs: changelog for ${HOTFIX_VERSION}"
```

### Step 4: Create PR and Release

```bash
# Push hotfix branch
git push origin hotfix-${HOTFIX_VERSION}

# Create PR to main
gh pr create --title "Hotfix ${HOTFIX_VERSION}" --base main

# After merge, tag and release (see Steps 5-7 above)
git checkout main
git pull upstream main
git tag -s ${HOTFIX_VERSION} -m "Hotfix ${HOTFIX_VERSION}"
git push upstream ${HOTFIX_VERSION}
```

## Rollback Procedure

If a release has critical issues:

### Step 1: Identify the Issue

Document the problem and verify it's caused by the new release.

### Step 2: Decide on Approach

**Option A: Hotfix** (for minor issues)
- Follow hotfix procedure above

**Option B: Rollback** (for major issues)
- Revert problematic commits
- Create new patch release

### Step 3: Communicate

- Update GitHub release with warning
- Notify users via community channels
- Document the issue in release notes

### Step 4: Execute Rollback

```bash
# Create revert branch
git checkout -b revert-${VERSION} main

# Revert the problematic commits
git revert <commit-hash>

# Create new patch release with reverts
export ROLLBACK_VERSION="v0.6.2"
# Follow normal release process
```

## Release Checklist Summary

Use this checklist for each release:

- [ ] All tests pass
- [ ] Documentation updated
- [ ] CHANGELOG created
- [ ] Version numbers updated
- [ ] Release branch created and PR opened
- [ ] PR approved and merged
- [ ] Tag created and pushed
- [ ] GitHub release created
- [ ] Artifacts verified
- [ ] Release announced
- [ ] Next milestone created

## Related Documentation

- [Development Guide](development.md)
- [Testing Guide](testing.md)
- [Contributing Guide](../../CONTRIBUTING.md)
- [CI/CD Workflows](../../.github/workflows/)

## Questions?

If you have questions about the release process:
1. Check existing release PRs for examples
2. Ask in the developer Slack channel
3. Contact the release team

---

**Note:** This guide is continuously updated. If you find issues or have suggestions, please open a PR to improve it!
