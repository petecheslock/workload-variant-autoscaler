# Multi-Model Migration Guide

This guide helps you migrate from a single-model WVA installation to a multi-model architecture that supports multiple llm-d stacks across different namespaces.

## Overview

**Prior to v0.4.3**, the WVA Helm chart installed both the controller and model-specific resources together. This meant that installing WVA for a new model would overwrite the resources from existing models, making it impossible to support multiple llm-d stacks.

**Starting with v0.4.3**, WVA supports three installation modes that enable you to decouple the controller from model resources:

- `all` (default) - Install both controller and model resources together
- `controller-only` - Install only the WVA controller
- `model-resources-only` - Install only model-specific resources

## When to Migrate

You should consider migrating to the new multi-model architecture if you:

- Have multiple llm-d stacks in different namespaces
- Want to add models without affecting existing models
- Need to scale different models independently
- Want to manage model lifecycles separately from the controller

## Migration Steps

### Step 1: Document Current Configuration

Before starting the migration, document your current setup:

```bash
# List existing WVA installations
helm ls -A | grep workload-variant-autoscaler

# Save current VariantAutoscaling resources
kubectl get variantautoscaling -A -o yaml > /tmp/existing-va.yaml

# Save current HPA resources
kubectl get hpa -A | grep vllm > /tmp/existing-hpa.txt

# Save current model configuration
kubectl get variantautoscaling -A -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.namespace}{"\t"}{.spec.modelID}{"\n"}{end}' > /tmp/models.txt
```

### Step 2: Backup Current Installation

Create a backup of your current Helm values:

```bash
# Get current values
helm get values workload-variant-autoscaler -n workload-variant-autoscaler-system > /tmp/current-values.yaml
```

### Step 3: Uninstall Old Installation

Remove the existing WVA installation:

```bash
# Uninstall WVA
helm uninstall workload-variant-autoscaler -n workload-variant-autoscaler-system

# Verify resources are removed
kubectl get pods -n workload-variant-autoscaler-system
kubectl get variantautoscaling -A
kubectl get hpa -A | grep vllm
```

**Note**: The VariantAutoscaling and HPA resources will be deleted. This is expected as we will recreate them in the next steps.

### Step 4: Install WVA Controller (Cluster-Wide)

Install the WVA controller once for the entire cluster:

```bash
# Install controller in controller-only mode
helm install wva-controller ./charts/workload-variant-autoscaler \
  -n workload-variant-autoscaler-system \
  --create-namespace \
  --set installMode=controller-only \
  --set wva.namespaceScoped=false \
  --set wva.prometheus.baseURL="<your-prometheus-url>" \
  --set wva.prometheus.tls.insecureSkipVerify=<true-or-false> \
  --set-file wva.prometheus.caCert=<path-to-ca-cert>  # If needed

# Verify controller is running
kubectl get pods -n workload-variant-autoscaler-system
kubectl logs -n workload-variant-autoscaler-system -l app.kubernetes.io/name=workload-variant-autoscaler
```

### Step 5: Deploy Model Resources for Each Model

For each llm-d stack/model, install the model-specific resources:

#### Example: Model A

```bash
helm install wva-model-a ./charts/workload-variant-autoscaler \
  --set installMode=model-resources-only \
  --set llmd.namespace=llm-d-model-a \
  --set llmd.modelName=ms-model-a-llm-d-modelservice \
  --set llmd.modelID="meta-llama/Llama-2-7b-hf" \
  --set va.enabled=true \
  --set va.accelerator=H100 \
  --set va.sloTpot=10 \
  --set va.sloTtft=1000 \
  --set hpa.enabled=true \
  --set hpa.maxReplicas=10 \
  --set vllmService.enabled=true \
  --set vllmService.nodePort=30000 \
  --set vllmService.interval=15s
```

#### Example: Model B

```bash
helm install wva-model-b ./charts/workload-variant-autoscaler \
  --set installMode=model-resources-only \
  --set llmd.namespace=llm-d-model-b \
  --set llmd.modelName=ms-model-b-llm-d-modelservice \
  --set llmd.modelID="mistralai/Mistral-7B-v0.1" \
  --set va.enabled=true \
  --set va.accelerator=A100 \
  --set va.sloTpot=8 \
  --set va.sloTtft=800 \
  --set hpa.enabled=true \
  --set hpa.maxReplicas=8 \
  --set vllmService.enabled=true \
  --set vllmService.nodePort=30001 \
  --set vllmService.interval=15s
```

**Tip**: Use the information from Step 1 to configure each model correctly.

### Step 6: Verify Migration

Verify that all components are working correctly:

```bash
# Check controller
kubectl get pods -n workload-variant-autoscaler-system
kubectl logs -n workload-variant-autoscaler-system -l app.kubernetes.io/name=workload-variant-autoscaler

# Check model resources for each namespace
kubectl get variantautoscaling -A
kubectl get hpa -A | grep vllm
kubectl get service -A | grep vllm
kubectl get servicemonitor -A | grep vllm

# Check that the controller is watching all namespaces
kubectl logs -n workload-variant-autoscaler-system -l app.kubernetes.io/name=workload-variant-autoscaler | grep "Starting workers"

# Verify autoscaling is working
kubectl describe variantautoscaling -n llm-d-model-a
kubectl describe hpa -n llm-d-model-a
```

## Post-Migration Architecture

After migration, your architecture will look like this:

```
Cluster
├── workload-variant-autoscaler-system (namespace)
│   └── wva-controller (Deployment) ← Single controller watching all namespaces
│
├── llm-d-model-a (namespace)
│   ├── llm-d resources (Gateway, Scheduler, vLLM)
│   └── wva-resources
│       ├── VariantAutoscaling
│       ├── HPA
│       ├── Service
│       └── ServiceMonitor
│
├── llm-d-model-b (namespace)
│   ├── llm-d resources (Gateway, Scheduler, vLLM)
│   └── wva-resources
│       ├── VariantAutoscaling
│       ├── HPA
│       ├── Service
│       └── ServiceMonitor
│
└── llm-d-model-c (namespace)
    ├── llm-d resources (Gateway, Scheduler, vLLM)
    └── wva-resources
        ├── VariantAutoscaling
        ├── HPA
        ├── Service
        └── ServiceMonitor
```

## Adding New Models

After migration, adding new models is straightforward:

```bash
# Just install model resources for the new model
helm install wva-model-new ./charts/workload-variant-autoscaler \
  --set installMode=model-resources-only \
  --set llmd.namespace=llm-d-model-new \
  --set llmd.modelName=ms-model-new-llm-d-modelservice \
  --set llmd.modelID="new/model-id" \
  --set va.accelerator=<accelerator-type>
```

The new model resources won't affect existing models!

## Removing Models

To remove a model without affecting others:

```bash
# Uninstall model resources
helm uninstall wva-model-a

# Verify only that model's resources are removed
kubectl get variantautoscaling -A
kubectl get hpa -A | grep vllm
```

The controller and other models remain unaffected.

## Troubleshooting

### Controller Not Watching All Namespaces

**Problem**: Controller only watches its own namespace.

**Solution**: Ensure `wva.namespaceScoped=false` was set during controller installation:

```bash
# Check controller arguments
kubectl get deployment -n workload-variant-autoscaler-system workload-variant-autoscaler-controller-manager -o yaml | grep watch-namespace

# If --watch-namespace is present, reinstall with correct settings
helm uninstall wva-controller -n workload-variant-autoscaler-system
helm install wva-controller ./charts/workload-variant-autoscaler \
  -n workload-variant-autoscaler-system \
  --create-namespace \
  --set installMode=controller-only \
  --set wva.namespaceScoped=false \
  --set wva.prometheus.baseURL="<your-prometheus-url>"
```

### Model Resources Not Reconciling

**Problem**: VariantAutoscaling resources are not being reconciled.

**Solution**: 
1. Check controller logs for errors
2. Verify the controller has RBAC permissions for the model namespace
3. Ensure the VariantAutoscaling resource is correctly configured

```bash
# Check controller logs
kubectl logs -n workload-variant-autoscaler-system -l app.kubernetes.io/name=workload-variant-autoscaler | grep ERROR

# Check RBAC
kubectl auth can-i get variantautoscalings --as=system:serviceaccount:workload-variant-autoscaler-system:workload-variant-autoscaler-controller-manager -n llm-d-model-a
```

### HPA Shows Unknown Metrics

**Problem**: HPA shows `<unknown>` for the external metric.

**Solution**:
1. Verify Prometheus Adapter is installed and configured
2. Check that the VariantAutoscaling resource is emitting metrics
3. Verify the HPA metric selector matches the VariantAutoscaling metric labels

```bash
# Check external metrics API
kubectl get --raw "/apis/external.metrics.k8s.io/v1beta1/namespaces/llm-d-model-a/inferno_desired_replicas" | jq

# Check VariantAutoscaling status
kubectl describe variantautoscaling -n llm-d-model-a
```

## Rollback

If you need to rollback to the old installation method:

```bash
# Uninstall new components
helm uninstall wva-controller -n workload-variant-autoscaler-system
helm uninstall wva-model-a
helm uninstall wva-model-b
# ... uninstall all model releases

# Reinstall using the old method (installMode=all)
helm install workload-variant-autoscaler ./charts/workload-variant-autoscaler \
  -n workload-variant-autoscaler-system \
  --create-namespace \
  --set installMode=all \
  -f /tmp/current-values.yaml  # Use your saved values
```

## Benefits of Multi-Model Architecture

After migration, you'll benefit from:

- **Isolation**: Each model's resources are independent
- **Flexibility**: Add/remove models without affecting others
- **Scalability**: Scale models independently based on their workload
- **Simplicity**: Simpler management with one controller for all models
- **Reliability**: Failure in one model's resources doesn't affect others

## Related Documentation

- [Installation Guide](installation.md)
- [Configuration Guide](configuration.md)
- [Helm Chart README](../../charts/workload-variant-autoscaler/README.md)
- [Deployment Guide](../../deploy/README.md)
