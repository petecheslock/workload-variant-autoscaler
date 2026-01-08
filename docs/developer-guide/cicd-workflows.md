# CI/CD Workflows

Comprehensive guide to the Continuous Integration and Continuous Deployment workflows used in the Workload-Variant-Autoscaler repository.

## Overview

WVA uses GitHub Actions for automated testing, building, and releasing. All workflows are defined in `.github/workflows/` and follow infrastructure-as-code principles.

## Workflow Categories

### Continuous Integration Workflows

#### PR Checks (`ci-pr-checks.yaml`)

**Trigger**: Every pull request to `main` or `dev` branches  
**Purpose**: Validate code quality and functionality before merge

**Pipeline Steps**:
1. **Code Checkout**: Clone repository with full history
2. **Go Setup**: Extract Go version from `go.mod` and configure environment
3. **Dependency Installation**: Download Go modules
4. **Linting**: Run `golangci-lint` with repository configuration
5. **Build Verification**: Execute `make build` to ensure compilation succeeds
6. **Unit Tests**: Run `make test` for all unit tests
7. **E2E Tests**: Deploy Kind cluster and run `make test-e2e` for saturation-based tests

**Success Criteria**:
- All linting checks pass
- Build completes without errors
- All unit tests pass
- E2E tests complete successfully

#### OpenShift E2E Tests (`ci-e2e-openshift.yaml`)

**Trigger**: Manual dispatch or specific PR labels  
**Purpose**: Validate WVA on real OpenShift clusters with GPU hardware

**Pipeline Steps**:
1. **Environment Setup**: Configure OpenShift credentials and namespace
2. **Infrastructure Deployment**: Deploy WVA controller and dependencies
3. **Test Execution**: Run ShareGPT workload tests with real vLLM servers
4. **Metrics Validation**: Verify autoscaling behavior and Prometheus metrics
5. **Smart Cleanup**: On failure, scale down GPU workloads while preserving debug resources (VA, HPA, logs)

**Test Configuration**:
- Namespace: `llm-d-inference-scheduling`
- Model: `unsloth/Meta-Llama-3.1-8B`
- Request rate: 20 req/s
- Prompts: 3000 total
- Token limit: 400 (sustains GPU load)

**Resource Management**:
The workflow intelligently handles failures by freeing GPU resources for other PRs without manual intervention, while keeping debugging artifacts intact.

See [test/e2e-openshift/README.md](../../test/e2e-openshift/README.md) for detailed test documentation.

### Release Workflows

#### Container Image Release (`ci-release.yaml`)

**Trigger**: Push of version tags (e.g., `v0.4.1`)  
**Purpose**: Build and publish multi-architecture Docker images

**Pipeline Steps**:
1. **Checkout**: Fetch source code at tag reference
2. **Version Extraction**: Parse semantic version from git tag
3. **QEMU Setup**: Configure multi-architecture builds
4. **Docker Login**: Authenticate to GitHub Container Registry (GHCR)
5. **Multi-Arch Build**: Build for `linux/amd64` and `linux/arm64`
6. **Image Push**: Publish to `ghcr.io/llm-d-incubation/workload-variant-autoscaler`

**Image Tags**:
- Semantic version: `v0.4.1`
- Latest: `latest` (always points to most recent release)

#### Helm Chart Release (`helm-release.yaml`)

**Trigger**: GitHub release publication, manual dispatch, or test tags (`v*-test`)  
**Purpose**: Package and publish Helm charts to GHCR OCI registry

**Pipeline Steps**:

1. **Checkout and Version Determination**
   - Parse version from tag (e.g., `v0.4.1` → `0.4.1`)
   - Support manual testing via workflow dispatch

2. **Docker Image Build**
   - Set up QEMU for multi-architecture support
   - Build `linux/amd64` and `linux/arm64` images
   - Push to `ghcr.io/llm-d-incubation/workload-variant-autoscaler`

3. **Helm Chart Updates**
   - Update `Chart.yaml`: Set `version` and `appVersion`
   - Update `values.yaml`: Set `wva.image.tag` to release version
   - Generate updated documentation with `helm-docs`

4. **Helm Chart Packaging**
   - Package chart as `.tgz` artifact
   - Login to GHCR OCI registry
   - Push to `oci://ghcr.io/llm-d-incubation/charts`

5. **Commit Chart Metadata**
   - Commit updated Chart.yaml and values.yaml
   - Push changes back to release tag
   - Update chart metadata in repository

**OCI Registry**:
Charts are published to: `oci://ghcr.io/llm-d-incubation/charts/workload-variant-autoscaler`

**Installation Example**:
```bash
helm install workload-variant-autoscaler \
  oci://ghcr.io/llm-d-incubation/charts/workload-variant-autoscaler \
  --version 0.4.1 \
  --namespace workload-variant-autoscaler-system
```

### Automation Workflows

#### Update Docs (`update-docs.md`)

**Trigger**: Every push to `main` branch  
**Purpose**: AI-powered documentation synchronization

This agentic workflow:
- Analyzes code changes to identify documentation gaps
- Updates documentation following Diátaxis framework
- Creates draft pull requests for documentation changes
- Maintains consistent style and terminology

**Style Guidelines**:
- Precise, concise, developer-friendly
- Active voice, plain English
- Progressive disclosure (high-level first, details second)

See [Agentic Workflows Guide](agentic-workflows.md#update-docs) for details.

#### Assign Docs PR (`assign-docs-pr.yml`)

**Trigger**: Documentation PR opened by `github-actions[bot]`  
**Purpose**: Assign documentation PRs to original PR authors

**Workflow Logic**:
1. Detect documentation PRs (title starts with `docs:`)
2. Extract original PR number from body or branch name
3. Assign docs PR to original PR author (not merger)
4. Add attribution comment for transparency
5. Fallback to most recent merged PR if metadata missing

This ensures accountability and makes it clear who should review documentation updates.

See [Agentic Workflows Guide](agentic-workflows.md#assign-docs-pr-to-original-author) for details.

#### Issue Labeler (`labeler.yaml`)

**Trigger**: New issues opened  
**Purpose**: Automatically triage incoming issues

**Configuration**: `.github/labeler.yaml`

All new issues receive the `needs-triage` label for team review.

### Prow Integration Workflows

#### Prow Commands (`prow-github.yaml`)

**Trigger**: Issue or PR comments  
**Purpose**: Enable Kubernetes Prow-style commands via GitHub Actions

**Available Commands**:
- `/assign [username]` - Assign issue/PR to user
- `/unassign [username]` - Unassign user
- `/approve` - Approve PR
- `/lgtm` - Mark as "looks good to me"
- `/retitle [new-title]` - Change issue/PR title
- `/area [label]` - Add area label
- `/kind [label]` - Add kind label
- `/priority [label]` - Add priority label
- `/remove [label]` - Remove label
- `/close` - Close issue/PR
- `/reopen` - Reopen issue/PR
- `/lock` - Lock conversation
- `/milestone [name]` - Set milestone
- `/hold` - Add do-not-merge hold
- `/cc [username]` - Request review
- `/uncc [username]` - Remove review request

**Label Configuration**: `.prowlabels.yaml`

#### PR Auto-Merge (`prow-pr-automerge.yaml`)

**Trigger**: PR receives `/lgtm` and `/approve` commands  
**Purpose**: Automatically merge approved PRs

#### Remove LGTM (`prow-pr-remove-lgtm.yaml`)

**Trigger**: New commits pushed to PR  
**Purpose**: Remove LGTM label when PR changes after approval

## Running Workflows Locally

### Simulate PR Checks

```bash
# Run full CI pipeline locally
make lint
make build
make test
make test-e2e
```

### Test Docker Build

```bash
# Build container image locally
docker build -t wva-test:local .

# Multi-architecture build (requires buildx)
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t wva-test:local .
```

### Test Helm Chart Packaging

```bash
# Package chart locally
cd charts/workload-variant-autoscaler
helm package .

# Validate chart
helm lint .
helm template test . --debug
```

## Workflow Secrets

Required GitHub secrets for workflows:

| Secret | Purpose | Used By |
|--------|---------|---------|
| `CR_TOKEN` | GitHub Container Registry authentication | Release workflows |
| `CR_USER` | GHCR username | Release workflows |
| `GITHUB_TOKEN` | Automatic GitHub Actions token | All workflows |

## Workflow Permissions

All workflows follow principle of least privilege:

| Workflow | Permissions |
|----------|-------------|
| `ci-pr-checks.yaml` | `read-all` |
| `ci-e2e-openshift.yaml` | `read-all` |
| `ci-release.yaml` | `contents: write`, `packages: write` |
| `helm-release.yaml` | `contents: write`, `packages: write` |
| `update-docs.md` | `read-all`, `pull-requests: write` (via safe-outputs) |
| `assign-docs-pr.yml` | `pull-requests: write`, `contents: read` |
| `labeler.yaml` | `issues: write`, `contents: read` |
| `prow-github.yaml` | `issues: write`, `pull-requests: write` |

## Debugging Workflows

### View Workflow Runs

```bash
# List recent workflow runs
gh run list --workflow=ci-pr-checks.yaml --limit 10

# View specific run details
gh run view <run-id>

# Download logs
gh run view <run-id> --log
```

### Re-run Failed Workflows

```bash
# Re-run failed jobs
gh run rerun <run-id> --failed

# Re-run entire workflow
gh run rerun <run-id>
```

### Manual Workflow Dispatch

```bash
# Trigger helm release workflow manually
gh workflow run helm-release.yaml -f tag=v0.4.1

# Trigger OpenShift E2E tests
gh workflow run ci-e2e-openshift.yaml
```

## Best Practices

### Workflow Development

1. **Test Locally First**: Run commands locally before pushing workflow changes
2. **Use Reusable Actions**: Extract common steps into composite actions
3. **Minimize Secrets**: Use `GITHUB_TOKEN` when possible, avoid custom PATs
4. **Add Comments**: Document complex workflow logic
5. **Version Actions**: Pin action versions for reproducibility (e.g., `@v4`)

### CI Optimization

1. **Cache Dependencies**: Use caching for Go modules, Docker layers
2. **Parallel Execution**: Run independent jobs in parallel
3. **Fast Feedback**: Run quick checks (lint, format) before expensive tests
4. **Fail Fast**: Stop workflow on first critical failure

### Release Management

1. **Semantic Versioning**: Follow semver for all releases (e.g., `v0.4.1`)
2. **Release Notes**: Include comprehensive changelog in GitHub releases
3. **Test Before Release**: Validate release artifacts on test environments
4. **Rollback Plan**: Document rollback procedures for failed releases

## Monitoring and Alerting

### Workflow Status

Check workflow status at: `https://github.com/llm-d-incubation/workload-variant-autoscaler/actions`

### Common Failure Patterns

| Symptom | Likely Cause | Resolution |
|---------|--------------|------------|
| Lint failures | Code style violations | Run `make lint-fix` locally |
| Test failures | Breaking changes | Review test output, fix code |
| Build failures | Compilation errors | Run `make build` locally |
| E2E timeout | Cluster resource constraints | Increase timeout or resources |
| Image push failure | Registry authentication | Check CR_TOKEN secret |
| Chart push failure | OCI registry access | Verify GHCR permissions |

## Related Documentation

- [Testing Guide](testing.md) - Comprehensive testing documentation
- [Development Guide](development.md) - Local development setup
- [Agentic Workflows Guide](agentic-workflows.md) - AI-powered automation
- [Release Process](releasing.md) - Release procedures and checklist

---

**Note**: Workflows are continuously improved. Submit issues or PRs for workflow enhancements.
