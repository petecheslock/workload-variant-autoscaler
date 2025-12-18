# Installation Guide

This guide covers installing Workload-Variant-Autoscaler (WVA) on your Kubernetes cluster.

> **New in v0.4.3**: WVA now supports flexible installation modes for multi-model deployments. See the [Multi-Model Migration Guide](multi-model-migration.md) for details on deploying WVA across multiple llm-d stacks.

## Prerequisites

- Kubernetes v1.32.0 or later
- Helm 3.x
- kubectl configured to access your cluster
- Cluster admin privileges

## Installation Methods

### Option 1: Helm Installation (Recommended)

The simplest way to install WVA is using Helm. The Helm chart supports three installation modes:

- **`all` (default)**: Install both controller and model resources together
- **`controller-only`**: Install only the controller for cluster-wide management
- **`model-resources-only`**: Install only model-specific resources

**Basic installation (single model):**

```bash
# Install WVA with default configuration
helm install workload-variant-autoscaler ./charts/workload-variant-autoscaler \
  --namespace workload-variant-autoscaler-system \
  --create-namespace

# Or with custom values
helm install workload-variant-autoscaler ./charts/workload-variant-autoscaler \
  --namespace workload-variant-autoscaler-system \
  --create-namespace \
  --values custom-values.yaml
```

**Multi-model installation:**

For deploying WVA to manage multiple models across different namespaces:

```bash
# Step 1: Install controller once
helm install wva-controller ./charts/workload-variant-autoscaler \
  -n workload-variant-autoscaler-system \
  --create-namespace \
  --set installMode=controller-only \
  --set wva.namespaceScoped=false

# Step 2: Install model resources for each model
helm install wva-model-a ./charts/workload-variant-autoscaler \
  --set installMode=model-resources-only \
  --set llmd.namespace=llm-d-model-a \
  --set llmd.modelName=model-a \
  --set llmd.modelID="meta-llama/Llama-2-7b-hf"
```

See the [Multi-Model Migration Guide](multi-model-migration.md) for complete details.

**Verify the installation:**
```bash
kubectl get pods -n workload-variant-autoscaler-system
```

### Option 2: Kustomize Installation

Using kustomize for more control:

```bash
# Install CRDs
make install

# Deploy the controller
make deploy IMG=quay.io/llm-d/workload-variant-autoscaler:latest
```

### Option 3: Using Installation Scripts

For specific platforms:

**Kubernetes:**
```bash
cd deploy/kubernetes
./install.sh
```

**OpenShift:**
```bash
cd deploy/openshift
./install.sh
```

**Local Development (Kind Emulator):**
```bash
# See deploy/kind-emulator/README.md for detailed instructions
make deploy-llm-d-wva-emulated-on-kind
```

## Configuration

### Helm Values

Key configuration options:

```yaml
# custom-values.yaml
image:
  repository: quay.io/llm-d/workload-variant-autoscaler
  tag: latest
  pullPolicy: IfNotPresent

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi

# Enable Prometheus monitoring
prometheus:
  enabled: true
  servicemonitor:
    enabled: true
```

### ConfigMaps

WVA uses ConfigMaps for cluster configuration:

- **Accelerator Unit Cost**: GPU pricing information
- **Service Classes**: SLO definitions for different service tiers

See [Configuration Guide](configuration.md) for details.

## Integrating with HPA/KEDA

WVA can work with existing autoscalers:

**For HPA integration:**
See [HPA Integration Guide](../integrations/hpa-integration.md)

**For KEDA integration:**
See [KEDA Integration Guide](../integrations/keda-integration.md)

## Verifying Installation

1. **Check controller is running:**
   ```bash
   kubectl get deployment -n workload-variant-autoscaler-system
   ```

2. **Verify CRDs are installed:**
   ```bash
   kubectl get crd variantautoscalings.llmd.ai
   ```

3. **Check controller logs:**
   ```bash
   kubectl logs -n workload-variant-autoscaler-system \
     deployment/workload-variant-autoscaler-controller-manager
   ```

## Uninstallation

**Helm:**
```bash
helm uninstall workload-variant-autoscaler -n workload-variant-autoscaler-system
```

**Kustomize:**
```bash
make undeploy
make uninstall  # Remove CRDs
```

## Troubleshooting

### Common Issues

**Controller not starting:**
- Check if CRDs are installed: `kubectl get crd`
- Verify RBAC permissions
- Check controller logs for errors

**Metrics not appearing:**
- Ensure Prometheus ServiceMonitor is created
- Verify Prometheus has proper RBAC to scrape metrics
- Check network policies aren't blocking metrics endpoint

**See Also:**
- [Configuration Guide](configuration.md)
- [Troubleshooting Guide](troubleshooting.md) (coming soon)
- [Developer Guide](../developer-guide/development.md)

## Next Steps

- [Configure your first VariantAutoscaling resource](configuration.md)
- [Follow the Quick Start Demo](../tutorials/demo.md)
- [Set up integration with HPA](../integrations/hpa-integration.md)

