# Frequently Asked Questions (FAQ)

Common questions about Workload-Variant-Autoscaler (WVA).

## Table of Contents

- [General Questions](#general-questions)
- [Architecture and Design](#architecture-and-design)
- [Installation and Setup](#installation-and-setup)
- [Configuration](#configuration)
- [Autoscaling Behavior](#autoscaling-behavior)
- [Metrics and Monitoring](#metrics-and-monitoring)
- [Integrations](#integrations)
- [Performance and Optimization](#performance-and-optimization)
- [Troubleshooting](#troubleshooting)

## General Questions

### What is Workload-Variant-Autoscaler?

WVA is a Kubernetes controller that performs intelligent autoscaling for LLM inference workloads based on saturation. It monitors vLLM server metrics (KV cache utilization, queue depth) and determines optimal replica counts to maintain performance while minimizing infrastructure costs.

### How is WVA different from standard HPA?

Standard HPA scales based on simple metrics like CPU or memory utilization. WVA:
- Uses LLM-specific metrics (KV cache, queue depth, request rate)
- Understands saturation patterns of inference workloads
- Optimizes for both cost and SLO compliance
- Works with existing HPA/KEDA for actual scaling execution

### What inference servers does WVA support?

Currently, WVA is designed for and tested with vLLM inference servers. Support for other inference servers may be added in the future.

### Does WVA require GPUs?

For production use with real LLM workloads, yes. However, WVA provides an emulation mode for development and testing that works on clusters without GPUs (including local Kind clusters on Mac/Windows).

### What license is WVA under?

Apache 2.0. See [LICENSE](../../LICENSE) for details.

## Architecture and Design

### How does WVA work?

WVA follows a pipeline architecture:
1. **Collector**: Gathers metrics from Prometheus (vLLM KV cache, queue depth, request rates)
2. **Analyzer**: Analyzes saturation using capacity models
3. **Optimizer**: Determines optimal replica count based on thresholds
4. **Actuator**: Emits metrics to Prometheus and updates VariantAutoscaling status
5. **External Autoscaler**: HPA or KEDA reads metrics and scales deployment

### Why doesn't WVA scale deployments directly?

WVA follows Kubernetes best practices by:
- Separating concerns (recommendation vs. execution)
- Leveraging existing, battle-tested autoscaling infrastructure (HPA/KEDA)
- Enabling flexible policy configuration through HPA behavior settings
- Allowing gradual rollout and easier troubleshooting

### What metrics does WVA use?

**Input metrics** (from vLLM):
- `vllm:gpu_cache_usage_perc` - KV cache utilization
- `vllm:num_requests_waiting` - Queue depth
- Request rates and throughput

**Output metrics** (to HPA/KEDA):
- `inferno_current_replicas` - Current replica count
- `inferno_desired_replicas` - Recommended replica count
- `inferno_desired_ratio` - Scale ratio

### What is the saturation model?

WVA uses a capacity-based saturation model that:
- Monitors KV cache utilization and queue depth
- Identifies when servers approach saturation (high queue depth, high cache usage)
- Provides slack capacity to handle traffic bursts
- Scales proactively before performance degrades

See [Saturation Scaling Configuration](../saturation-scaling-config.md) for details.

### Does WVA support multiple models per cluster?

Yes! You can create multiple VariantAutoscaling resources, each targeting a different deployment. Each resource operates independently with its own configuration and metrics.

## Installation and Setup

### What are the minimum requirements?

- Kubernetes v1.31.0+ (or OpenShift 4.18+)
- Helm 3.x
- Prometheus (for metrics collection)
- For production: GPU-enabled cluster

### Can I run WVA locally without GPUs?

Yes! Use the Kind emulator mode:
```bash
make deploy-llm-d-wva-emulated-on-kind
```

This creates a local Kind cluster with emulated GPUs. Works on Mac (Apple Silicon/Intel) and Windows. See [Kind Emulator Guide](../../deploy/kind-emulator/README.md).

### Do I need to deploy llm-d infrastructure?

For the full automated deployment (including vLLM), yes. However, if you have existing vLLM deployments, you can:
1. Install WVA controller only via Helm
2. Configure Prometheus to scrape your vLLM metrics
3. Create VariantAutoscaling resources pointing to your deployments

### How do I upgrade WVA?

```bash
# Important: Apply updated CRDs first (Helm doesn't auto-update CRDs)
kubectl apply -f charts/workload-variant-autoscaler/crds/

# Then upgrade Helm release
helm upgrade workload-variant-autoscaler ./charts/workload-variant-autoscaler \
  -n workload-variant-autoscaler-system
```

See [Installation Guide](installation.md#upgrading) for details.

### Can I use WVA with existing Prometheus?

Yes! Configure the Prometheus connection in values.yaml:
```yaml
wva:
  prometheus:
    baseURL: "https://your-prometheus:9090"
    tls:
      caCertPath: "/path/to/ca.crt"
```

## Configuration

### What configuration is required?

At minimum, specify the model ID:
```yaml
apiVersion: llmd.ai/v1alpha1
kind: VariantAutoscaling
metadata:
  name: my-autoscaler
spec:
  modelId: "meta/llama-3.1-8b"
```

WVA will infer the target deployment from the model ID.

### How do I target a specific deployment?

Use `scaleTargetRef`:
```yaml
spec:
  modelId: "meta/llama-3.1-8b"
  scaleTargetRef:
    name: my-vllm-deployment
    kind: Deployment
```

### What are service classes?

Service classes define performance tiers with different SLO targets. Configure them via ConfigMap:
```yaml
serviceClasses:
  premium:
    sloTtft: 500  # ms
    sloTpot: 5    # ms
  standard:
    sloTtft: 1000
    sloTpot: 10
```

Then reference in VariantAutoscaling:
```yaml
spec:
  serviceClassName: "premium"
```

### How do I configure GPU costs?

Edit the accelerator costs ConfigMap:
```yaml
accelerators:
  H100:
    unitCost: 4.0
  A100:
    unitCost: 2.5
  L40S:
    unitCost: 1.8
```

### Can I adjust scaling behavior?

Yes, through HPA configuration. See [HPA Integration Guide](../integrations/hpa-integration.md) for:
- Stabilization windows
- Scale-up/down policies
- Min/max replicas

## Autoscaling Behavior

### When should I create the VariantAutoscaling resource?

You can create it before or after the target deployment:
- **Before deployment**: WVA gracefully handles missing deployments
- **After deployment**: WVA starts optimizing immediately once deployment is running

### How long does it take to scale?

Typical timeline:
1. WVA detects need to scale: 60s (default reconciliation interval)
2. Prometheus scrapes updated metrics: 15-30s
3. HPA reads metrics and makes decision: 15s
4. Kubernetes scales deployment: 10-60s (depends on pod startup time)

Total: ~2-5 minutes for scale-up, longer for scale-down (due to stabilization windows).

### Why is scaling slow?

By design! Gradual scaling prevents:
- Thrashing (rapid scale up/down cycles)
- Unnecessary pod churn
- Cost waste from over-scaling

Adjust if needed via `reconcileInterval` and HPA `stabilizationWindowSeconds`.

### Can WVA scale to zero?

Yes, but it's experimental. Enable in values.yaml:
```yaml
wva:
  scaleToZero: true
```

Note: Scale-from-zero requires additional cold-start handling in your application.

### What happens if deployment is deleted?

WVA immediately updates the VariantAutoscaling status to reflect the missing deployment. When the deployment is recreated, WVA automatically resumes operation.

### Does WVA handle node scaling?

No. WVA scales pods, not nodes. Use Cluster Autoscaler or Karpenter for node-level autoscaling based on pod resource requests.

## Metrics and Monitoring

### How do I view WVA metrics?

```bash
# Port forward to controller
kubectl port-forward -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager 8443:8443

# View metrics (accept self-signed cert warning)
curl -k https://localhost:8443/metrics
```

Or use Prometheus/Grafana dashboards.

### What metrics should I monitor?

Key metrics:
- `inferno_current_replicas` - Current replica count
- `inferno_desired_replicas` - WVA recommendation
- `inferno_saturation_level` - Current saturation (0-1)
- `inferno_reconcile_duration_seconds` - Controller performance

See [Metrics Guide](../metrics-health-monitoring.md) for complete list.

### How do I debug why autoscaling isn't working?

1. Check VariantAutoscaling status:
   ```bash
   kubectl describe variantautoscaling <name> -n <namespace>
   ```

2. View controller logs:
   ```bash
   kubectl logs -n workload-variant-autoscaler-system \
     deployment/workload-variant-autoscaler-controller-manager -f
   ```

3. Check HPA status:
   ```bash
   kubectl describe hpa <name> -n <namespace>
   ```

See [Troubleshooting Guide](troubleshooting.md) for detailed steps.

## Integrations

### Do I need both HPA and KEDA?

No, choose one:
- **HPA**: Native Kubernetes, simpler setup, recommended for most use cases
- **KEDA**: More flexible triggers, better for event-driven scaling

See:
- [HPA Integration Guide](../integrations/hpa-integration.md)
- [KEDA Integration Guide](../integrations/keda-integration.md)

### Can I use custom metrics?

Yes! WVA emits metrics to Prometheus. You can:
1. Create Prometheus recording rules
2. Configure Prometheus Adapter to expose them
3. Use in HPA or KEDA ScaledObject

### Does WVA work with Istio?

Yes! The deployment scripts support Istio as a gateway provider:
```bash
export GATEWAY_PROVIDER="istio"
```

### Does WVA work on OpenShift?

Yes! Fully supported. Use the OpenShift deployment script:
```bash
cd deploy/openshift
./install.sh
```

See [OpenShift Deployment Guide](../../deploy/openshift/README.md).

## Performance and Optimization

### How much overhead does WVA add?

Minimal:
- CPU: ~100-200m (default)
- Memory: ~128-256Mi (default)
- Metrics collection: 1 Prometheus query per reconciliation cycle

### Can I optimize WVA for large clusters?

Yes:
1. Increase reconciliation interval (reduce query frequency)
2. Use Prometheus recording rules (pre-compute queries)
3. Scale controller vertically (more CPU/memory)
4. Use label selectors to limit watched resources

### Does WVA affect inference latency?

No. WVA operates out-of-band and doesn't sit in the request path. It only:
- Reads metrics from Prometheus
- Updates Kubernetes resources
- Scales deployments through HPA/KEDA

## Troubleshooting

### WVA controller won't start

Check:
1. CRDs installed: `kubectl get crd variantautoscalings.llmd.ai`
2. RBAC permissions: `kubectl get clusterrolebinding | grep wva`
3. Prometheus connection: verify URL and TLS config
4. Controller logs for specific errors

See [Troubleshooting Guide](troubleshooting.md#controller-not-starting).

### Metrics show `<unknown>` in HPA

Prometheus Adapter issue. Check:
1. Prometheus Adapter is running
2. Adapter configuration has correct metric rules
3. VariantAutoscaling resource has correct labels
4. Metrics are available in Prometheus

See [Troubleshooting Guide](troubleshooting.md#metrics-not-appearing-in-prometheus).

### Scaling is too aggressive/slow

Adjust HPA behavior:
```yaml
behavior:
  scaleUp:
    stabilizationWindowSeconds: 60  # Faster (default: 240)
  scaleDown:
    stabilizationWindowSeconds: 600  # Slower (default: 240)
```

See [HPA Integration Guide](../integrations/hpa-integration.md#controlling-scaling-behavior).

### Where can I find more help?

- [Troubleshooting Guide](troubleshooting.md) - Detailed problem-solving steps
- [GitHub Issues](https://github.com/llm-d-incubation/workload-variant-autoscaler/issues) - Report bugs or ask questions
- [Community Meetings](https://github.com/llm-d-incubation/workload-variant-autoscaler) - Join discussions
- [Contributing Guide](../../CONTRIBUTING.md) - Get involved

## Contributing

### Can I contribute to WVA?

Absolutely! See:
- [Contributing Guide](../../CONTRIBUTING.md)
- [Developer Guide](../developer-guide/development.md)
- [Testing Guide](../developer-guide/testing.md)

### How do I report bugs?

Open an issue on [GitHub Issues](https://github.com/llm-d-incubation/workload-variant-autoscaler/issues) with:
- WVA version
- Kubernetes/OpenShift version
- Steps to reproduce
- Expected vs. actual behavior
- Relevant logs

## See Also

- [Installation Guide](installation.md)
- [Configuration Guide](configuration.md)
- [Troubleshooting Guide](troubleshooting.md)
- [Main README](../../README.md)
- [Architecture Documentation](../design/modeling-optimization.md)
