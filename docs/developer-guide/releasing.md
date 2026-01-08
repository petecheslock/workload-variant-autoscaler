# Releasing WVA

This guide covers the release process for the Workload-Variant-Autoscaler (WVA).

## Overview

WVA uses semantic versioning (MAJOR.MINOR.PATCH) and follows a release process that includes:
- Version tagging
- Container image builds
- Helm chart releases
- Release notes and documentation

## Prerequisites

Before starting a release, ensure you have:

- Maintainer access to the repository
- Write access to the container registry
- GitHub CLI (`gh`) installed and authenticated
- Helm CLI installed
- Git configured with your commit signing key (if required)

## Release Types

### Patch Release (x.y.Z)

Bug fixes and minor improvements that don't change the API or behavior.

**Example**: v0.4.1 → v0.4.2

**Process**:
1. Create a release branch from main
2. Cherry-pick bug fixes
3. Update version numbers
4. Tag and release

### Minor Release (x.Y.0)

New features, enhancements, and non-breaking changes.

**Example**: v0.4.2 → v0.5.0

**Process**:
1. Ensure all planned features are merged
2. Update documentation
3. Update version numbers
4. Tag and release

### Major Release (X.0.0)

Breaking changes, major architectural updates, API changes.

**Example**: v0.5.0 → v1.0.0

**Process**:
1. Complete migration guide
2. Update all documentation
3. Communicate breaking changes widely
4. Tag and release

## Release Checklist

### Pre-Release

- [ ] All planned features/fixes are merged to main
- [ ] CI/CD pipeline passes on main branch
- [ ] All tests pass locally and in CI
- [ ] Documentation is up-to-date
  - [ ] README.md reflects current version
  - [ ] CHANGELOG.md is updated
  - [ ] API documentation matches code
  - [ ] User guides reference correct version
- [ ] CRD changes are documented
- [ ] Helm chart version is updated
- [ ] Breaking changes are clearly documented

### Version Updates

Update version numbers in these files:

1. **Helm Chart** (`charts/workload-variant-autoscaler/Chart.yaml`):
   ```yaml
   version: 0.5.0  # Chart version
   appVersion: "0.5.0"  # Application version
   ```

2. **Makefile** (if applicable):
   ```makefile
   VERSION ?= 0.5.0
   ```

3. **Container Image Tags** (in CI/CD workflows):
   ```yaml
   - name: Build and push
     env:
       VERSION: v0.5.0
   ```

### Release Process

#### 1. Create Release Branch (for major/minor releases)

```bash
# Create and push release branch
git checkout main
git pull origin main
git checkout -b release-v0.5.0
git push origin release-v0.5.0
```

#### 2. Update Version Numbers

```bash
# Update Chart.yaml
vim charts/workload-variant-autoscaler/Chart.yaml

# Update other version references
grep -r "version.*0.4.0" .

# Commit changes
git add .
git commit -s -m "chore: bump version to v0.5.0"
git push origin release-v0.5.0
```

#### 3. Update CHANGELOG

Create or update `CHANGELOG.md`:

```markdown
## [0.5.0] - 2026-01-08

### Added
- Feature: Scale-from-zero support for idle deployments
- Enhancement: Multi-controller isolation with label selectors
- Documentation: Comprehensive FAQ and troubleshooting guides

### Changed
- Improved saturation analysis algorithm for better accuracy
- Updated Prometheus query efficiency

### Fixed
- Bug: Incorrect replica calculation under high load
- Issue #123: Controller crash when deployment is deleted

### Breaking Changes
- VariantAutoscaling CRD: Added required `scaleTargetRef` field
  - Migration: Existing CRs will infer target from `modelID` for backward compatibility
```

Commit the changelog:
```bash
git add CHANGELOG.md
git commit -s -m "docs: update CHANGELOG for v0.5.0"
git push origin release-v0.5.0
```

#### 4. Create Release Pull Request

```bash
# Create PR from release branch to main
gh pr create \
  --title "Release v0.5.0" \
  --body "Release v0.5.0 - See CHANGELOG.md for details" \
  --base main \
  --head release-v0.5.0
```

#### 5. Tag the Release

After the release PR is merged:

```bash
# Checkout and pull main
git checkout main
git pull origin main

# Create and push tag
git tag -a v0.5.0 -m "Release v0.5.0"
git push origin v0.5.0
```

#### 6. Automated Release Build

The CI/CD pipeline (`.github/workflows/ci-release.yaml`) automatically:
- Builds container images
- Pushes to container registry
- Creates Helm chart package
- Publishes Helm chart

Monitor the GitHub Actions workflow:
```bash
gh workflow view ci-release
gh run watch
```

#### 7. Create GitHub Release

Create the release with notes:

```bash
gh release create v0.5.0 \
  --title "Workload-Variant-Autoscaler v0.5.0" \
  --notes-file release-notes.md \
  --verify-tag
```

**Release notes template**:
```markdown
# Workload-Variant-Autoscaler v0.5.0

## Highlights

- 🚀 New scale-from-zero support for idle deployments
- 🎯 Multi-controller isolation for enterprise deployments
- 📚 Comprehensive documentation updates

## Installation

### Helm Chart
\`\`\`bash
helm upgrade -i workload-variant-autoscaler \
  oci://ghcr.io/llm-d-incubation/charts/workload-variant-autoscaler \
  --version 0.5.0 \
  --namespace workload-variant-autoscaler-system \
  --create-namespace
\`\`\`

### Upgrading from v0.4.x

⚠️ **Important**: Manually apply CRD updates before upgrading:

\`\`\`bash
kubectl apply -f https://github.com/llm-d-incubation/workload-variant-autoscaler/releases/download/v0.5.0/crds.yaml
\`\`\`

See the [Upgrade Guide](docs/user-guide/installation.md#upgrading) for details.

## What's Changed

### Added
- Feature: Scale-from-zero engine (#456)
- Enhancement: Multi-controller label selectors (#478)
- Documentation: FAQ and troubleshooting guides (#490)

### Changed
- Improved saturation algorithm accuracy (#467)
- Updated Prometheus query performance (#472)

### Fixed
- Controller crash on deployment deletion (#445)
- Incorrect replicas under high load (#461)

## Breaking Changes

### VariantAutoscaling CRD: Required `scaleTargetRef` Field

The `scaleTargetRef` field is now required in the VariantAutoscaling spec:

\`\`\`yaml
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-model-deployment
  modelID: "meta/llama-3.1-8b"
\`\`\`

**Migration**: Existing CRs without `scaleTargetRef` will continue to work in v0.5.0 by inferring the target from `modelID`. However, this behavior will be deprecated in v0.6.0.

See [Migration Guide](docs/user-guide/migration-v0.5.md) for detailed migration steps.

## Full Changelog

[v0.4.2...v0.5.0](https://github.com/llm-d-incubation/workload-variant-autoscaler/compare/v0.4.2...v0.5.0)

## Contributors

Thank you to all contributors who made this release possible!

@contributor1, @contributor2, @contributor3
```

#### 8. Publish Helm Chart

If using a Helm chart repository:

```bash
# Package chart
helm package charts/workload-variant-autoscaler

# Push to OCI registry
helm push workload-variant-autoscaler-0.5.0.tgz oci://ghcr.io/llm-d-incubation/charts

# Or update Helm repository index
helm repo index . --url https://llm-d-incubation.github.io/workload-variant-autoscaler
```

### Post-Release

- [ ] Verify release artifacts are available
  - [ ] Container images are pullable
  - [ ] Helm chart is installable
  - [ ] GitHub release is published
- [ ] Announce release
  - [ ] Post in community Slack/Discord
  - [ ] Update documentation website
  - [ ] Send announcement to mailing list (if applicable)
- [ ] Update main branch
  - [ ] Bump version to next development version (e.g., v0.5.1-dev)
  - [ ] Update documentation to reference new stable version

## Hotfix Releases

For critical bugs in production:

1. **Create hotfix branch from release tag**:
   ```bash
   git checkout -b hotfix-v0.5.1 v0.5.0
   ```

2. **Apply fixes and test**:
   ```bash
   # Cherry-pick or apply fixes
   git cherry-pick <commit-hash>
   
   # Test thoroughly
   make test
   ```

3. **Update version to patch release**:
   ```bash
   # Update Chart.yaml and other files
   vim charts/workload-variant-autoscaler/Chart.yaml
   git commit -s -m "chore: bump version to v0.5.1"
   ```

4. **Tag and release**:
   ```bash
   git push origin hotfix-v0.5.1
   git tag -a v0.5.1 -m "Hotfix v0.5.1"
   git push origin v0.5.1
   ```

5. **Merge back to main**:
   ```bash
   git checkout main
   git merge hotfix-v0.5.1
   git push origin main
   ```

## Release Automation

The release process uses GitHub Actions workflows:

- **`ci-release.yaml`**: Builds and publishes container images and Helm charts
- **`helm-release.yaml`**: Publishes Helm charts to OCI registry

### Triggering Automated Release

Releases are triggered by pushing version tags:

```bash
git tag v0.5.0
git push origin v0.5.0
```

The workflow:
1. Builds multi-arch container images
2. Runs security scans (Trivy)
3. Pushes images to ghcr.io
4. Packages Helm chart
5. Publishes chart to OCI registry
6. Creates GitHub release (if configured)

## Versioning Policy

WVA follows [Semantic Versioning 2.0.0](https://semver.org/):

- **MAJOR** (X.0.0): Incompatible API changes, breaking changes
- **MINOR** (x.Y.0): New features, backward-compatible changes
- **PATCH** (x.y.Z): Bug fixes, backward-compatible fixes

### Pre-releases

Use pre-release versions for testing:

- **Alpha**: `v0.5.0-alpha.1` - Early development, unstable
- **Beta**: `v0.5.0-beta.1` - Feature complete, testing phase
- **RC**: `v0.5.0-rc.1` - Release candidate, final testing

## Troubleshooting

### Failed Release Build

Check GitHub Actions logs:
```bash
gh run list --workflow=ci-release
gh run view <run-id> --log-failed
```

Common issues:
- **Container push fails**: Check registry credentials
- **Helm chart validation fails**: Run `helm lint` locally
- **Tests fail**: Run `make test` locally to reproduce

### Reverting a Release

If a critical issue is found post-release:

1. **Immediately tag and release a fixed version**
2. **Update GitHub release notes with warning**
3. **Announce the issue and fixed version**

Do NOT delete tags or releases - version history must be immutable.

## Best Practices

- **Test releases in staging environment first**
- **Communicate breaking changes early and clearly**
- **Maintain backward compatibility within major versions**
- **Write clear, comprehensive release notes**
- **Coordinate releases with related projects (llm-d infrastructure)**
- **Plan releases around community meeting schedules**

## Related Documentation

- [Contributing Guide](../../CONTRIBUTING.md)
- [Development Setup](development.md)
- [CI/CD Workflows](../../.github/workflows/)
- [Semantic Versioning](https://semver.org/)

---

For questions about the release process, open a discussion or contact the maintainers.
