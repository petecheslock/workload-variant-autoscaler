# Release Process

Comprehensive guide for releasing new versions of Workload-Variant-Autoscaler.

## Overview

WVA follows semantic versioning and uses automated GitHub workflows for building, packaging, and publishing releases. This guide covers the complete release process from preparation through post-release validation.

## Release Types

### Patch Releases (v0.4.x)

**Purpose**: Bug fixes, security patches, documentation updates  
**Frequency**: As needed  
**Breaking Changes**: ❌ No

Example: `v0.4.1` → `v0.4.2`

### Minor Releases (v0.x.0)

**Purpose**: New features, enhancements, CRD additions  
**Frequency**: Monthly to quarterly  
**Breaking Changes**: ⚠️ Possible (with deprecation notice)

Example: `v0.4.1` → `v0.5.0`

### Major Releases (vx.0.0)

**Purpose**: Major architecture changes, API redesigns  
**Frequency**: Annually or as needed  
**Breaking Changes**: ✅ Yes

Example: `v0.4.1` → `v1.0.0`

## Prerequisites

### Required Access

- [ ] Write access to `llm-d-incubation/workload-variant-autoscaler` repository
- [ ] Permission to create GitHub releases
- [ ] Access to GitHub Container Registry (GHCR)
- [ ] Maintainer status in the project

### Required Tools

```bash
# GitHub CLI
gh version

# Docker with buildx
docker buildx version

# Helm
helm version

# kubectl (for validation)
kubectl version --client

# Git
git --version
```

### Environment Setup

```bash
# Authenticate with GitHub
gh auth login

# Authenticate with GHCR
echo $CR_TOKEN | docker login ghcr.io -u $CR_USER --password-stdin
```

## Release Checklist

### 1. Pre-Release Preparation

#### Update Dependencies

```bash
# Update Go dependencies
go get -u ./...
go mod tidy

# Run tests to verify compatibility
make test
make test-e2e
```

#### Update Documentation

- [ ] Update README.md with new features/changes
- [ ] Update user guide for new functionality
- [ ] Update CRD reference if API changed
- [ ] Review and update architecture diagrams
- [ ] Update CHANGELOG.md (create if doesn't exist)

#### Version Bump

Determine the next version based on semantic versioning:

```bash
# Current version
CURRENT_VERSION="v0.4.1"

# Next version (example: minor release)
NEXT_VERSION="v0.5.0"
```

#### Update Chart Files

Update `charts/workload-variant-autoscaler/Chart.yaml`:

```yaml
version: 0.5.0        # Remove 'v' prefix for Helm
appVersion: "v0.5.0"  # Keep 'v' prefix for git tag
```

**Note**: The release workflow will automatically update these files, but manually updating them ensures consistency.

### 2. Create Release Branch (Optional)

For major/minor releases, consider creating a release branch:

```bash
# Create and push release branch
git checkout -b release/v0.5.0
git push origin release/v0.5.0
```

For patch releases, typically release directly from `main`.

### 3. Final Testing

```bash
# Run full test suite
make test

# Run linter
make lint

# Run E2E tests on Kind
make test-e2e

# Build and verify binary
make build
./bin/manager --version

# Build and test Docker image locally
docker build -t wva-test:local .
```

#### Optional: Test on Real Cluster

```bash
# Deploy to test cluster
export KUBECONFIG=/path/to/test/cluster
make deploy

# Create test VariantAutoscaling resource
kubectl apply -f config/samples/variantautoscaling-with-cost.yaml

# Verify controller behavior
kubectl logs -n workload-variant-autoscaler-system deployment/workload-variant-autoscaler-controller-manager

# Clean up
make undeploy
```

### 4. Create Git Tag

```bash
# Create annotated tag
git tag -a v0.5.0 -m "Release v0.5.0"

# Verify tag
git show v0.5.0

# Push tag to trigger workflows
git push origin v0.5.0
```

**Important**: Pushing the tag triggers the automated release workflows:
- `ci-release.yaml` - Builds and pushes Docker images
- `helm-release.yaml` - Packages and publishes Helm chart

### 5. Create GitHub Release

#### Option A: Via GitHub Web UI

1. Navigate to: `https://github.com/llm-d-incubation/workload-variant-autoscaler/releases/new`
2. Select the tag: `v0.5.0`
3. Set release title: `v0.5.0 - Release Name`
4. Add release notes (see template below)
5. Check "Set as the latest release" if applicable
6. Click "Publish release"

#### Option B: Via GitHub CLI

```bash
# Create release with notes
gh release create v0.5.0 \
  --title "v0.5.0 - Saturation-Based Scaling Enhancements" \
  --notes-file RELEASE_NOTES.md

# Or generate notes automatically
gh release create v0.5.0 \
  --title "v0.5.0" \
  --generate-notes
```

### 6. Monitor Release Workflows

```bash
# Watch workflow progress
gh run list --workflow=helm-release.yaml --limit 5

# View specific run
gh run view <run-id> --log

# Check for failures
gh run list --status=failure --limit 10
```

## Release Notes Template

```markdown
## What's Changed

### Features
- Add saturation-based scaling engine ([#123](https://github.com/llm-d-incubation/workload-variant-autoscaler/pull/123))
- Support for multi-controller isolation ([#124](https://github.com/llm-d-incubation/workload-variant-autoscaler/pull/124))

### Bug Fixes
- Fix KV cache utilization calculation ([#125](https://github.com/llm-d-incubation/workload-variant-autoscaler/pull/125))
- Resolve HPA metric aggregation issue ([#126](https://github.com/llm-d-incubation/workload-variant-autoscaler/pull/126))

### Documentation
- Add CI/CD workflows guide ([#127](https://github.com/llm-d-incubation/workload-variant-autoscaler/pull/127))
- Update installation guide with Helm 3.14+ support ([#128](https://github.com/llm-d-incubation/workload-variant-autoscaler/pull/128))

### Breaking Changes
- **VariantAutoscaling CRD**: Added required `scaleTargetRef` field. Existing resources must be updated.
  
  **Migration Guide**:
  ```yaml
  # Before (v0.4.x)
  apiVersion: llmd.ai/v1alpha1
  kind: VariantAutoscaling
  metadata:
    name: example
  spec:
    modelID: "model-name"
  
  # After (v0.5.0)
  apiVersion: llmd.ai/v1alpha1
  kind: VariantAutoscaling
  metadata:
    name: example
  spec:
    scaleTargetRef:
      apiVersion: apps/v1
      kind: Deployment
      name: model-deployment
    modelID: "model-name"
  ```

### Upgrade Notes

**CRD Updates Required**: This release includes CRD schema changes. Manually apply CRDs before upgrading:

```bash
kubectl apply -f charts/workload-variant-autoscaler/crds/

helm upgrade workload-variant-autoscaler \
  oci://ghcr.io/llm-d-incubation/charts/workload-variant-autoscaler \
  --version 0.5.0 \
  --namespace workload-variant-autoscaler-system
```

### Deprecations

- `experimentalHybridOptimization` flag is deprecated and will be removed in v0.6.0. Use the new saturation-based scaling engine instead.

## Installation

### Docker Image

```bash
docker pull ghcr.io/llm-d-incubation/workload-variant-autoscaler:v0.5.0
```

### Helm Chart

```bash
helm install workload-variant-autoscaler \
  oci://ghcr.io/llm-d-incubation/charts/workload-variant-autoscaler \
  --version 0.5.0 \
  --namespace workload-variant-autoscaler-system \
  --create-namespace
```

## Full Changelog

**Full Changelog**: https://github.com/llm-d-incubation/workload-variant-autoscaler/compare/v0.4.1...v0.5.0

---

**Contributors**: Thanks to all contributors who made this release possible! 🎉
@contributor1, @contributor2, @contributor3
```

## Post-Release Validation

### 1. Verify Docker Images

```bash
# Pull and inspect images
docker pull ghcr.io/llm-d-incubation/workload-variant-autoscaler:v0.5.0

# Verify both architectures
docker manifest inspect ghcr.io/llm-d-incubation/workload-variant-autoscaler:v0.5.0

# Test image locally
docker run --rm ghcr.io/llm-d-incubation/workload-variant-autoscaler:v0.5.0 --version
```

### 2. Verify Helm Chart

```bash
# Search for chart
helm search repo workload-variant-autoscaler --versions

# Pull chart from OCI registry
helm pull oci://ghcr.io/llm-d-incubation/charts/workload-variant-autoscaler --version 0.5.0

# Extract and inspect
tar -xzf workload-variant-autoscaler-0.5.0.tgz
cat workload-variant-autoscaler/Chart.yaml
```

### 3. Test Installation

```bash
# Install on test cluster
helm install wva-test \
  oci://ghcr.io/llm-d-incubation/charts/workload-variant-autoscaler \
  --version 0.5.0 \
  --namespace wva-test \
  --create-namespace \
  --dry-run

# Actual installation (if dry-run succeeds)
helm install wva-test \
  oci://ghcr.io/llm-d-incubation/charts/workload-variant-autoscaler \
  --version 0.5.0 \
  --namespace wva-test \
  --create-namespace

# Verify deployment
kubectl get deployment -n wva-test
kubectl get pods -n wva-test
kubectl logs -n wva-test -l control-plane=controller-manager

# Clean up test installation
helm uninstall wva-test -n wva-test
kubectl delete namespace wva-test
```

### 4. Update Documentation Sites

If applicable:
- [ ] Update GitHub Pages documentation
- [ ] Update external documentation sites
- [ ] Notify community channels

## Rollback Procedure

If critical issues are discovered post-release:

### Option 1: Quick Fix (Patch Release)

```bash
# Fix the issue in code
git checkout main
# ... make fixes ...
git commit -m "fix: critical bug in v0.5.0"

# Create patch release
git tag -a v0.5.1 -m "Release v0.5.1 - Hotfix"
git push origin v0.5.1

# Create GitHub release
gh release create v0.5.1 --title "v0.5.1 - Hotfix" --generate-notes
```

### Option 2: Release Deprecation

```bash
# Mark release as non-latest
gh release edit v0.5.0 --draft=false --prerelease

# Add warning to release notes
gh release edit v0.5.0 --notes "⚠️ **DEPRECATED**: This release has critical issues. Use v0.4.1 instead."
```

### Option 3: Rollback Installation

For users encountering issues:

```bash
# Rollback Helm release
helm rollback workload-variant-autoscaler -n workload-variant-autoscaler-system

# Or install previous version explicitly
helm upgrade workload-variant-autoscaler \
  oci://ghcr.io/llm-d-incubation/charts/workload-variant-autoscaler \
  --version 0.4.1 \
  --namespace workload-variant-autoscaler-system
```

## Post-Release Tasks

### Immediate (Within 24 Hours)

- [ ] Monitor GitHub issues for release-related problems
- [ ] Check workflow run status for any failures
- [ ] Verify Docker image availability on GHCR
- [ ] Verify Helm chart availability on OCI registry
- [ ] Announce release in community channels

### Short-Term (Within 1 Week)

- [ ] Update dependent repositories (if any)
- [ ] Write blog post or release announcement
- [ ] Update roadmap with completed features
- [ ] Plan next release cycle
- [ ] Archive release branch (for major/minor releases)

### Communication Channels

Announce the release on:
- GitHub Discussions
- Slack workspace
- Community meetings
- Mailing lists (if applicable)

## Troubleshooting

### Workflow Failures

#### Docker Build Fails

**Problem**: Multi-arch build fails with platform errors

**Solution**:
```bash
# Verify buildx is configured
docker buildx ls

# Create new builder if needed
docker buildx create --name multiarch --use
docker buildx inspect --bootstrap
```

#### Helm Chart Push Fails

**Problem**: OCI registry authentication failure

**Solution**:
```bash
# Verify GHCR access
echo $CR_TOKEN | helm registry login ghcr.io -u $CR_USER --password-stdin

# Test push manually
helm package charts/workload-variant-autoscaler
helm push workload-variant-autoscaler-0.5.0.tgz oci://ghcr.io/llm-d-incubation/charts
```

#### CRD Schema Errors

**Problem**: CRD fails validation after changes

**Solution**:
```bash
# Regenerate CRDs
make manifests

# Validate locally
kubectl apply --dry-run=server -f config/crd/bases/
```

### Common Release Issues

| Issue | Cause | Resolution |
|-------|-------|------------|
| Tag already exists | Previous release with same tag | Delete tag: `git tag -d v0.5.0 && git push origin :refs/tags/v0.5.0` |
| Workflow doesn't trigger | Tag not pushed | Verify: `git ls-remote --tags origin` |
| Image build timeout | Large context size | Add `.dockerignore` file |
| Chart version mismatch | Manual edit error | Update `Chart.yaml` manually |
| Helm install fails | Missing CRDs | Apply CRDs first: `kubectl apply -f crds/` |

## Best Practices

### Version Numbering

- **MAJOR**: Incompatible API changes
- **MINOR**: Backward-compatible functionality additions
- **PATCH**: Backward-compatible bug fixes

### Release Cadence

- **Patch releases**: As needed for critical fixes
- **Minor releases**: Every 4-8 weeks for features
- **Major releases**: Annually or when significant breaking changes accumulate

### Deprecation Policy

1. Announce deprecation at least one minor version in advance
2. Document migration path in release notes
3. Remove deprecated features in next major version
4. Maintain backward compatibility within major versions

### Security Releases

For security vulnerabilities:
1. Coordinate with security team
2. Prepare patches for all supported versions
3. Create security advisory on GitHub
4. Release patches simultaneously across versions
5. Announce via security channels

## Release Automation Improvements

Future enhancements under consideration:
- [ ] Automated changelog generation from commit messages
- [ ] Release candidate (RC) workflow for beta testing
- [ ] Automated rollback on failed smoke tests
- [ ] Integration with security scanning in release pipeline
- [ ] Automated documentation versioning

## Related Documentation

- [CI/CD Workflows](cicd-workflows.md) - Detailed workflow documentation
- [Testing Guide](testing.md) - Test strategy and execution
- [Development Guide](development.md) - Development environment setup
- [Contributing Guide](../../CONTRIBUTING.md) - Contribution guidelines

---

**Questions?** Ask in GitHub Discussions or community Slack channel.
