# Frequently Asked Questions (FAQ)

## General Questions

### What is the Workload-Variant-Autoscaler (WVA)?

WVA is a Kubernetes controller that performs intelligent autoscaling for inference model servers based on real-time saturation metrics. It monitors KV cache utilization and queue depth to prevent capacity exhaustion while minimizing infrastructure costs.

### How is WVA different from HPA?

WVA analyzes inference-specific metrics (KV cache, queue depth) to determine optimal replica counts and emits recommendations. HPA reads these recommendations and performs the actual scaling. WVA provides intelligence; HPA provides execution.

Think of it as: **WVA = Brain, HPA/KEDA = Muscles**

### Do I need GPU hardware to use WVA?

No! WVA works with:
- **Real GPUs:** Production environments with physical GPUs
- **Emulated GPUs:** Local development with Kind clusters (no GPU required)
- **CPU-only:** Test environments without accelerators

The emulated mode is perfect for development and testing on Mac, Windows, or Linux without GPUs.

### Which inference frameworks does WVA support?

Currently, WVA is optimized for **vLLM** inference servers. Support for other frameworks (TensorRT-LLM, TGI) is planned.

### Can I use WVA with multiple models?

Yes! Create one VariantAutoscaling resource per model. Each model can have multiple variants (different accelerators) managed by a single VariantAutoscaling CR.

## Installation & Setup

### What are the prerequisites for installing WVA?

**Required:**
- Kubernetes 1.31.0+ (or OpenShift 4.18+)
- Helm 3.x
- kubectl
- Prometheus (for metrics)

**For production:**
- HPA or KEDA for scaling
- vLLM inference servers

**For development:**
- Kind (installed automatically by Makefile)
- Docker 17.03+

See the [Installation Guide](./installation.md) for details.

### How do I install WVA on my cluster?

**Simplest method (Helm):**
```bash
helm upgrade -i workload-variant-autoscaler ./charts/workload-variant-autoscaler \
  --namespace workload-variant-autoscaler-system \
  --create-namespace
```

**For local testing:**
```bash
make deploy-llm-d-wva-emulated-on-kind
```

This creates a complete environment with emulated GPUs, Prometheus, and sample deployments.

### Do I need to install Prometheus separately?

Yes, WVA requires Prometheus for metrics collection. However, our Helm chart and Kind deployment scripts include Prometheus setup.

### How do I upgrade WVA to a new version?

**Important:** Helm does not automatically upgrade CRDs. Apply CRDs manually first:

```bash
# Apply updated CRDs
kubectl apply -f charts/workload-variant-autoscaler/crds/

# Then upgrade Helm release
helm upgrade workload-variant-autoscaler ./charts/workload-variant-autoscaler \
  --namespace workload-variant-autoscaler-system
```

See the README "Upgrading" section for breaking changes.

## Configuration

### How do I configure saturation thresholds?

Edit the `capacity-scaling-config` ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: capacity-scaling-config
  namespace: workload-variant-autoscaler-system
data:
  default: |
    kvCacheThreshold: 0.80      # 80% KV cache triggers saturation
    queueLengthThreshold: 5     # 5 queued requests triggers saturation
    kvSpareTrigger: 0.1         # Scale up when spare < 10%
    queueSpareTrigger: 3        # Scale up when queue > 3 requests
```

Per-model overrides are also supported. See [Saturation Scaling Configuration](../saturation-scaling-config.md).

### Can I customize costs for different accelerators?

Yes! Configure the `accelerator-unitcost` ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: accelerator-unitcost
  namespace: workload-variant-autoscaler-system
data:
  costs: |
    A100: 30.0
    H100: 50.0
    L40S: 15.0
```

WVA will prefer cheaper accelerators when multiple variants can serve the same model.

### How do I enable scale-to-zero?

Configure HPA or KEDA to allow `minReplicas: 0`:

**HPA:**
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: my-model-hpa
spec:
  minReplicas: 0
  maxReplicas: 10
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
```

**KEDA:**
```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: my-model-scaler
spec:
  minReplicaCount: 0
  maxReplicaCount: 10
  cooldownPeriod: 300
```

See [HPA Integration](../integrations/hpa-integration.md) or [KEDA Integration](../integrations/keda-integration.md).

### What is the recommended HPA stabilization window?

We recommend **120-300 seconds** to prevent flapping:

```yaml
behavior:
  scaleUp:
    stabilizationWindowSeconds: 60
  scaleDown:
    stabilizationWindowSeconds: 180
```

Adjust based on your pod startup time and traffic patterns.

## Operations

### How do I check if WVA is working correctly?

1. **Check controller logs:**
```bash
kubectl logs -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager
```

2. **Verify VariantAutoscaling status:**
```bash
kubectl get variantautoscaling -A
kubectl describe variantautoscaling <name> -n <namespace>
```

3. **Check metrics in Prometheus:**
```bash
# Query WVA recommendations
inferno_desired_replicas

# Query vLLM metrics
vllm:kv_cache_usage_perc
vllm:num_requests_waiting
```

### Why isn't my deployment scaling?

**Checklist:**

1. **Is WVA reconciling the VariantAutoscaling resource?**
   - Check controller logs for reconciliation events
   - Verify `kubectl get variantautoscaling` shows your resource

2. **Are metrics available?**
   - Check `MetricsAvailable` condition in CR status
   - Verify Prometheus is reachable
   - Confirm vLLM pods are emitting metrics

3. **Is HPA/KEDA configured correctly?**
   - Check `kubectl get hpa` or `kubectl get scaledobject`
   - Verify metric target is set correctly (typically 1.0)
   - Check HPA/KEDA logs for errors

4. **Is the deployment selector correct?**
   - Verify `scaleTargetRef` in VariantAutoscaling matches deployment name
   - Check deployment exists in the same namespace

### Why do I see "MetricsAvailable: False"?

This means WVA cannot reach Prometheus or retrieve metrics.

**Troubleshooting:**

1. **Verify Prometheus is running:**
```bash
kubectl get pods -n <prometheus-namespace>
```

2. **Check Prometheus connectivity:**
```bash
# From WVA pod
kubectl exec -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager -- \
  curl -k https://prometheus-k8s.prometheus:9091/api/v1/query?query=up
```

3. **Verify TLS configuration:**
- Check `prometheus-ca-cert` ConfigMap exists
- Ensure CA certificate is valid

4. **Check controller logs for specific errors:**
```bash
kubectl logs -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager | grep -i prometheus
```

### How do I troubleshoot slow scaling?

**Factors affecting scaling speed:**

1. **WVA reconciliation interval** (default: 60s)
   - Adjust via `RECONCILIATION_INTERVAL` env var

2. **HPA sync period** (default: 15s)
   - Adjust via `--horizontal-pod-autoscaler-sync-period` flag

3. **HPA stabilization window** (default: 300s for scale-down)
   - Configure via `behavior.scaleDown.stabilizationWindowSeconds`

4. **Pod startup time**
   - Optimize image size and initialization
   - Use readiness probes appropriately

**Recommended settings for faster scaling:**
```yaml
# WVA
RECONCILIATION_INTERVAL: 30s

# HPA
behavior:
  scaleUp:
    stabilizationWindowSeconds: 0
    policies:
    - type: Percent
      value: 100
      periodSeconds: 15
  scaleDown:
    stabilizationWindowSeconds: 120
```

### Can I disable saturation-based scaling?

Saturation-based scaling is always active by default (it's WVA's primary mode). However, you can:

1. **Adjust thresholds** to be more or less aggressive
2. **Use model-based optimization** in hybrid mode (combines both approaches)
3. **Disable WVA entirely** and use only HPA with standard metrics

There's no configuration to disable saturation analysis while keeping WVA active.

## Multi-Variant Scenarios

### What are variants?

**Variants** are different configurations of the same model deployment, typically using different GPU types. For example:

- `llama-70b-a100` - Llama 70B on A100 GPUs
- `llama-70b-h100` - Llama 70B on H100 GPUs

WVA analyzes saturation across all variants and can recommend scaling the cheapest variant.

### How does WVA choose which variant to scale?

**Scale-up:** WVA scales the **cheapest** variant (lowest cost per replica)

**Scale-down:** WVA scales down the **most expensive** variant

Costs are configured in the `accelerator-unitcost` ConfigMap. If costs are equal, WVA uses alphabetical order (deterministic tie-breaking).

### Can I have multiple VariantAutoscaling resources for the same model?

No. Create **one** VariantAutoscaling resource per model. WVA automatically discovers all variants (deployments) for that model based on the `modelID` field.

### How do I create variants?

Deploy multiple instances of your model with different accelerator nodeSelectors:

```yaml
# Variant 1: A100
apiVersion: apps/v1
kind: Deployment
metadata:
  name: llama-8b-a100
  labels:
    model_id: meta/llama-3.1-8b
spec:
  template:
    spec:
      nodeSelector:
        nvidia.com/gpu.product: A100-SXM4-80GB

---
# Variant 2: L40S
apiVersion: apps/v1
kind: Deployment
metadata:
  name: llama-8b-l40s
  labels:
    model_id: meta/llama-3.1-8b
spec:
  template:
    spec:
      nodeSelector:
        nvidia.com/gpu.product: L40S
```

Then create a single VariantAutoscaling CR:

```yaml
apiVersion: llmd.ai/v1alpha1
kind: VariantAutoscaling
metadata:
  name: llama-8b-autoscaler
spec:
  modelId: "meta/llama-3.1-8b"
```

## Performance & Tuning

### What is the expected latency for WVA to detect saturation?

**Typical detection latency:** 60-90 seconds

- Reconciliation interval: 60s (configurable)
- Prometheus scrape interval: 15-30s
- Metrics aggregation: <1s

### How much overhead does WVA add?

**Resource overhead:**
- CPU: <5% on typical clusters (mostly during reconciliation)
- Memory: 50-100 MB + metrics cache
- Network: Minimal (Prometheus queries every 60s)

**Scaling latency overhead:**
- WVA decision: <1s
- Metric emission: <1s
- HPA reaction: 15s (HPA sync period)

Total overhead: ~15-20 seconds on top of pod startup time.

### Can WVA scale multiple models simultaneously?

Yes! WVA scales each VariantAutoscaling resource independently in parallel. There's no sequential processing or blocking.

### What's the maximum number of models WVA can manage?

WVA can manage hundreds of models without performance issues. Each reconciliation is independent and lightweight.

**Practical limits:**
- 100s of models: No issues
- 1000s of models: May need controller tuning (increase workers)
- 10,000+ models: Consider sharding across multiple WVA instances

## Advanced Topics

### Can I use WVA with service mesh (Istio, Linkerd)?

Yes! WVA is service mesh compatible. Ensure:

1. Prometheus can scrape metrics through the mesh
2. WVA controller can reach Prometheus
3. mTLS certificates are configured correctly

### Does WVA support multi-cluster deployments?

Not currently. WVA operates within a single cluster. For multi-cluster:

- Deploy separate WVA instances per cluster
- Use a central metrics aggregator (Thanos, Cortex)
- Consider federation for cross-cluster optimization (future feature)

### Can I customize the optimization algorithm?

Not directly via configuration. WVA uses:

1. **Saturation analysis** (always active)
2. **Queueing theory optimizer** (when model parameters configured)

To customize, you would need to:
- Fork the repository
- Modify `internal/saturation/analyzer.go` or `pkg/solver/`
- Rebuild and deploy custom image

Alternatively, contribute your algorithm back to the project!

### How does WVA handle pod disruptions?

WVA is resilient to pod disruptions:

1. **Missing metrics:** Exclude pods without metrics from analysis
2. **Partial data:** Use available replica metrics only
3. **No metrics:** Graceful degradation - maintain last known state

The `MetricsAvailable` condition reflects metric health.

### Can I test WVA without deploying models?

Yes! Use the vLLM emulator for testing:

```bash
# Deploy WVA with emulated infrastructure
make deploy-llm-d-wva-emulated-on-kind
```

This includes:
- Emulated vLLM pods (simulate metrics)
- Prometheus and monitoring stack
- Sample VariantAutoscaling resources

See [Local Development Guide](../developer-guide/development.md).

## Troubleshooting

### Common Error Messages

#### "Failed to get metrics from Prometheus"

**Cause:** Cannot connect to Prometheus or query failed

**Solution:**
1. Verify Prometheus URL in controller configuration
2. Check network connectivity
3. Validate TLS certificates
4. Check Prometheus logs for query errors

#### "No pods found for model"

**Cause:** No pods match the model ID

**Solution:**
1. Verify `modelId` in VariantAutoscaling matches pod labels
2. Check deployment selector and labels
3. Ensure pods are running and ready

#### "Scale target not found"

**Cause:** Deployment referenced in `scaleTargetRef` doesn't exist

**Solution:**
1. Verify deployment name and namespace
2. Check deployment exists: `kubectl get deployment <name> -n <namespace>`
3. Update `scaleTargetRef` in VariantAutoscaling if needed

#### "HPA not scaling to desired replicas"

**Cause:** HPA not reading WVA metrics correctly

**Solution:**
1. Verify Prometheus Adapter is running
2. Check metric registration: `kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1`
3. Verify HPA metric target: `kubectl describe hpa <name>`
4. Check metric value: `kubectl get --raw "/apis/custom.metrics.k8s.io/v1beta1/namespaces/<ns>/pods/*/inferno_desired_replicas"`

### Getting Help

If you're still stuck:

1. **Check logs:**
   - WVA controller logs
   - HPA/KEDA logs
   - Prometheus logs
   - vLLM pod logs

2. **Search documentation:**
   - [User Guide](./installation.md)
   - [Troubleshooting sections](./configuration.md#troubleshooting-configuration)
   - [Integration guides](../integrations/)

3. **Open an issue:**
   - GitHub: https://github.com/llm-d-incubation/workload-variant-autoscaler/issues
   - Include: WVA version, logs, VariantAutoscaling YAML, error messages

4. **Join community:**
   - Slack: [llm-d autoscaling community](https://join.slack.com/share/...)

## Related Documentation

- [Installation Guide](./installation.md)
- [Configuration Guide](./configuration.md)
- [Architecture Overview](../design/architecture-overview.md)
- [HPA Integration](../integrations/hpa-integration.md)
- [KEDA Integration](../integrations/keda-integration.md)
- [Developer Guide](../developer-guide/development.md)
