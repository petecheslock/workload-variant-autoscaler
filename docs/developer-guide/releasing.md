# Releasing Guide

Guide for maintainers on how to create and publish new releases of Workload-Variant-Autoscaler.

## Table of Contents

- [Release Process Overview](#release-process-overview)
- [Pre-Release Checklist](#pre-release-checklist)
- [Version Numbering](#version-numbering)
- [Creating a Release](#creating-a-release)
- [Automated Release Workflow](#automated-release-workflow)
- [Post-Release Tasks](#post-release-tasks)
- [Helm Chart Releases](#helm-chart-releases)
- [Hotfix Releases](#hotfix-releases)
- [Rollback Procedures](#rollback-procedures)

## Release Process Overview

WVA follows semantic versioning and uses GitHub Actions for automated releases:

1. **Prepare release** - Update version numbers and CHANGELOG
2. **Create release branch** - Branch from main
3. **Run tests** - Ensure all CI checks pass
4. **Tag release** - Create Git tag
5. **Automated build** - GitHub Actions builds and publishes artifacts
6. **Publish Helm chart** - Chart is pushed to repository
7. **Update documentation** - Release notes and migration guides

## Pre-Release Checklist

Before creating a release, ensure:

### Code Quality
- [ ] All CI checks passing on main branch
- [ ] E2E tests passing on target environments (Kubernetes, OpenShift, Kind)
- [ ] No critical bugs or security issues
- [ ] Code coverage meets project standards

### Documentation
- [ ] CHANGELOG.md updated with all changes since last release
- [ ] Breaking changes documented in CHANGELOG
- [ ] Migration guide created (if breaking changes)
- [ ] API documentation up to date
- [ ] README.md version references updated

### Version Updates
- [ ] `charts/workload-variant-autoscaler/Chart.yaml` - `version` and `appVersion`
- [ ] `Makefile` - `IMG_TAG` default
- [ ] Documentation references to version numbers
- [ ] API version (if API changed)

### Testing
- [ ] Manual testing on at least one platform
- [ ] Upgrade testing from previous version
- [ ] Helm chart installation tested
- [ ] CRD migration tested (if CRDs changed)

## Version Numbering

WVA follows [Semantic Versioning 2.0.0](https://semver.org/):

- **MAJOR** (v1.0.0): Incompatible API changes
- **MINOR** (v0.5.0): New features, backward compatible
- **PATCH** (v0.4.2): Bug fixes, backward compatible

### Pre-release versions:
- **Alpha**: `v0.5.0-alpha.1` - Early development, unstable API
- **Beta**: `v0.5.0-beta.1` - Feature complete, testing phase
- **RC**: `v0.5.0-rc.1` - Release candidate, final testing

### Version Scheme

```
v{major}.{minor}.{patch}[-{pre-release}][+{build-metadata}]

Examples:
  v0.4.1          - Stable release
  v0.5.0-alpha.1  - Alpha pre-release
  v0.5.0-rc.2     - Release candidate
  v1.0.0+build.1  - With build metadata
```

## Creating a Release

### 1. Update Version Numbers

```bash
# Set version
export NEW_VERSION="v0.5.0"

# Update Chart.yaml
yq eval -i ".version = \"${NEW_VERSION#v}\"" charts/workload-variant-autoscaler/Chart.yaml
yq eval -i ".appVersion = \"$NEW_VERSION\"" charts/workload-variant-autoscaler/Chart.yaml

# Regenerate Helm chart README
cd charts/workload-variant-autoscaler
helm-docs
cd ../..

# Commit changes
git add charts/workload-variant-autoscaler/
git commit -m "chore: bump version to $NEW_VERSION"
```

### 2. Update CHANGELOG

```bash
# Edit CHANGELOG.md
vim CHANGELOG.md
```

Follow this format:

```markdown
## [0.5.0] - 2026-01-15

### Added
- New feature description
- Another feature

### Changed
- Breaking change with migration steps
- Behavior change

### Fixed
- Bug fix description

### Deprecated
- Feature to be removed

### Removed
- Removed feature (with migration guide link)

### Security
- Security fix description
```

### 3. Create Release Branch (Optional)

For major/minor releases:

```bash
git checkout -b release/v0.5.0
git push origin release/v0.5.0
```

For patch releases, typically release directly from main.

### 4. Create Git Tag

```bash
# Create annotated tag
git tag -a $NEW_VERSION -m "Release $NEW_VERSION"

# Push tag (triggers CI/CD)
git push origin $NEW_VERSION
```

## Automated Release Workflow

Pushing a tag triggers the `.github/workflows/ci-release.yaml` workflow:

### What the workflow does:

1. **Build multi-arch images**:
   - `linux/amd64`
   - `linux/arm64`

2. **Push to container registries**:
   - `ghcr.io/llm-d-incubation/workload-variant-autoscaler:$VERSION`
   - `ghcr.io/llm-d-incubation/workload-variant-autoscaler:latest`

3. **Security scanning**:
   - Trivy vulnerability scan
   - SARIF report uploaded to GitHub Security

4. **Create GitHub Release**:
   - Extract changelog for this version
   - Attach release artifacts
   - Mark as pre-release if alpha/beta/rc

5. **Update latest tag**:
   - For stable releases only (not pre-releases)

### Monitor Release Build

```bash
# Watch GitHub Actions
gh run watch

# Or visit: https://github.com/llm-d-incubation/workload-variant-autoscaler/actions
```

## Post-Release Tasks

### 1. Verify Artifacts

```bash
# Check image was published
docker pull ghcr.io/llm-d-incubation/workload-variant-autoscaler:$NEW_VERSION

# Verify multi-arch support
docker manifest inspect ghcr.io/llm-d-incubation/workload-variant-autoscaler:$NEW_VERSION

# Test installation
helm install wva-test ./charts/workload-variant-autoscaler \
  --set wva.image.tag=$NEW_VERSION \
  --dry-run
```

### 2. Update Documentation

- [ ] Update installation docs with new version
- [ ] Update quickstart examples
- [ ] Add migration guide if breaking changes
- [ ] Update compatibility matrix

### 3. Announce Release

- [ ] Create announcement in GitHub Discussions
- [ ] Update README badges (if major version)
- [ ] Notify community channels (Slack, mailing list)
- [ ] Tweet/blog post for major releases

### 4. Create Milestone for Next Release

```bash
# Via GitHub CLI
gh milestone create "v0.6.0" --description "Next minor release"
```

## Helm Chart Releases

The `.github/workflows/helm-release.yaml` workflow publishes Helm charts to GitHub Pages.

### Manual Helm Release

```bash
# Package chart
helm package charts/workload-variant-autoscaler

# Generate index
helm repo index . --url https://llm-d-incubation.github.io/workload-variant-autoscaler

# Commit to gh-pages branch
git checkout gh-pages
git add .
git commit -m "Release Helm chart $NEW_VERSION"
git push origin gh-pages
```

### Using the Chart

```bash
# Add repo
helm repo add wva https://llm-d-incubation.github.io/workload-variant-autoscaler

# Install
helm install workload-variant-autoscaler wva/workload-variant-autoscaler --version $NEW_VERSION
```

## Hotfix Releases

For critical bug fixes in production:

### 1. Create Hotfix Branch

```bash
# Branch from release tag
git checkout -b hotfix/v0.4.2 v0.4.1

# Apply fix
git cherry-pick <commit-sha>

# Or make changes directly
git commit -am "fix: critical bug description"
```

### 2. Bump Patch Version

```bash
export HOTFIX_VERSION="v0.4.2"

# Update Chart.yaml
yq eval -i ".version = \"${HOTFIX_VERSION#v}\"" charts/workload-variant-autoscaler/Chart.yaml
yq eval -i ".appVersion = \"$HOTFIX_VERSION\"" charts/workload-variant-autoscaler/Chart.yaml

git commit -am "chore: bump version to $HOTFIX_VERSION"
```

### 3. Test and Release

```bash
# Run critical tests
make test
make e2e-test

# Tag and push
git tag -a $HOTFIX_VERSION -m "Hotfix $HOTFIX_VERSION"
git push origin $HOTFIX_VERSION

# Merge back to main
git checkout main
git merge --no-ff hotfix/v0.4.2
git push origin main
```

## Rollback Procedures

If a release has critical issues:

### 1. Revert Image Tag

```bash
# Users can pin to previous version
helm upgrade workload-variant-autoscaler wva/workload-variant-autoscaler \
  --set wva.image.tag=v0.4.0
```

### 2. Delete Problematic Release (if necessary)

```bash
# Delete GitHub release
gh release delete $BAD_VERSION --yes

# Delete tag
git push --delete origin $BAD_VERSION
git tag -d $BAD_VERSION

# Delete image (requires admin access)
# Use GitHub Container Registry UI or:
# gh api -X DELETE /orgs/llm-d-incubation/packages/container/workload-variant-autoscaler/versions/$VERSION_ID
```

### 3. Issue New Release

Create hotfix with fix and new version number.

## Release Checklist Template

Copy this for each release:

```markdown
## Release v0.X.0 Checklist

### Pre-Release
- [ ] All CI checks passing
- [ ] E2E tests passing (Kubernetes, OpenShift, Kind)
- [ ] CHANGELOG updated
- [ ] Version numbers updated (Chart.yaml, docs)
- [ ] Breaking changes documented
- [ ] Migration guide created (if needed)
- [ ] Manual testing completed

### Release
- [ ] Git tag created and pushed
- [ ] GitHub Actions build succeeded
- [ ] Container images published
- [ ] Helm chart published
- [ ] GitHub Release created

### Post-Release
- [ ] Artifacts verified
- [ ] Documentation updated
- [ ] Announcement posted
- [ ] Milestone created for next release
- [ ] Old branches cleaned up

### Issues
- [ ] No blockers reported within 48 hours
```

## Security Considerations

- Never commit secrets or credentials
- Sign Git tags for releases: `git tag -s`
- Review security scan results before release
- Follow responsible disclosure for security issues
- Use GitHub Security Advisories for vulnerabilities

## Troubleshooting Release Issues

### Build Fails

```bash
# Check GitHub Actions logs
gh run view --log

# Test build locally
make docker-build IMG=test:local
```

### Image Not Found

```bash
# Verify image exists
docker manifest inspect ghcr.io/llm-d-incubation/workload-variant-autoscaler:$VERSION

# Check registry permissions
gh auth status
```

### Helm Chart Install Fails

```bash
# Validate chart
helm lint charts/workload-variant-autoscaler

# Test with dry-run
helm install wva-test charts/workload-variant-autoscaler --dry-run --debug
```

## Release Calendar

Suggested release cadence:

- **Minor releases**: Every 6-8 weeks
- **Patch releases**: As needed (bug fixes, security)
- **Major releases**: When breaking changes accumulate

## See Also

- [Contributing Guide](../../CONTRIBUTING.md)
- [Development Guide](development.md)
- [Testing Guide](testing.md)
- [CI/CD Workflows](../../.github/workflows/)
