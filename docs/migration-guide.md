# Migration Guide

## Removal of Performance Parameters from VariantAutoscaling CRD

### Overview

In recent updates, WVA has transitioned from a model-based approach using static performance parameters to a **metrics-based approach** using real-time Prometheus metrics. This change simplifies configuration and improves accuracy by using actual runtime behavior.

### What Changed

**Removed Fields:**
- `perfParms` - Performance parameters object
- `decodeParms.alpha` - Base decode latency parameter
- `decodeParms.beta` - Batch size decode overhead parameter  
- `prefillParms.gamma` - Base prefill latency parameter
- `prefillParms.delta` - Token count prefill overhead parameter

**Why This Change?**

1. **Eliminates Offline Benchmarking**: No need to run separate benchmarking jobs to estimate parameters
2. **Improved Accuracy**: Uses actual runtime metrics instead of predicted performance
3. **Simplified Configuration**: Fewer fields to configure in the CRD
4. **Dynamic Adaptation**: Automatically adapts to changing workload characteristics

### Migration Steps

#### Before (Old CRD Structure):

```yaml
apiVersion: llmd.ai/v1alpha1
kind: VariantAutoscaling
metadata:
  name: llama-8b-autoscaler
spec:
  scaleTargetRef:
    kind: Deployment
    name: llama-8b
  modelID: "meta/llama-3.1-8b"
  variantCost: "10.0"
  modelProfile:
    accelerators:
      - acc: "A100"
        accCount: 1
        perfParms:
          decodeParms:
            alpha: "20.5"
            beta: "0.41"
          prefillParms:
            gamma: "5.2"
            delta: "0.1"
        maxBatchSize: 256
```

#### After (Current CRD Structure):

```yaml
apiVersion: llmd.ai/v1alpha1
kind: VariantAutoscaling
metadata:
  name: llama-8b-autoscaler
spec:
  scaleTargetRef:
    kind: Deployment
    name: llama-8b
  modelID: "meta/llama-3.1-8b"
  variantCost: "10.0"
  modelProfile:
    accelerators:
      - acc: "A100"
        accCount: 1
        maxBatchSize: 256
```

**Key Changes:**
1. Remove the entire `perfParms` section
2. Keep `acc`, `accCount`, and `maxBatchSize`
3. All other fields remain the same

### How WVA Works Now

Instead of using static performance parameters, WVA now:

1. **Collects Real-Time Metrics**: Queries Prometheus for actual vLLM server metrics including:
   - KV cache utilization
   - Queue depth
   - Request rates
   - Token throughput
   - ITL and TTFT measurements

2. **Saturation-Based Scaling**: Makes scaling decisions based on actual saturation levels:
   - Monitors queue depth and cache usage
   - Scales up when servers approach saturation
   - Scales down when capacity is underutilized

3. **Cost Optimization**: Uses `variantCost` to prefer lower-cost variants when multiple options exist

### Prerequisites for the New Approach

Ensure your deployment has proper metrics collection configured:

1. **ServiceMonitor**: vLLM pods must be scraped by Prometheus
   ```yaml
   apiVersion: monitoring.coreos.com/v1
   kind: ServiceMonitor
   metadata:
     name: vllm-metrics
   spec:
     selector:
       matchLabels:
         app: vllm
     endpoints:
     - port: metrics
       interval: 15s
   ```

2. **Prometheus Access**: WVA controller must have access to query Prometheus
   - Configure Prometheus endpoint in WVA deployment
   - Ensure proper RBAC and network policies

3. **vLLM Metrics**: Ensure vLLM exposes Prometheus metrics on `/metrics` endpoint

See [Prometheus Integration Guide](./integrations/prometheus.md) for detailed setup.

### Frequently Asked Questions

**Q: Do I need to update my existing VariantAutoscaling resources?**

A: If you have `perfParms` in your CRD definitions, you should remove them. The controller will ignore these fields but it's best practice to clean them up.

**Q: Will my autoscaling behavior change?**

A: Yes, but for the better! The new approach is more accurate because it uses actual runtime metrics instead of estimated parameters. You may see:
- More responsive scaling (reacts to actual load)
- Better handling of variable workloads
- No need to re-benchmark when workload patterns change

**Q: What if I don't have Prometheus metrics?**

A: WVA requires Prometheus metrics to function. The controller will mark the VariantAutoscaling resource as not ready until metrics are available. See the [Prometheus Integration Guide](./integrations/prometheus.md).

**Q: Can I still use the parameter estimation guide?**

A: The [parameter estimation guide](./tutorials/parameter-estimation.md) has been marked as deprecated but is preserved for historical reference. It's no longer needed for WVA operation.

**Q: How does WVA handle cold starts now?**

A: WVA waits for metrics to be available before making scaling decisions. During initial deployment:
1. Deployment starts with initial replica count
2. WVA monitors for metrics availability
3. Once metrics are available, WVA begins making scaling recommendations
4. External autoscaler (HPA/KEDA) acts on the recommendations

### Related Documentation

- [Configuration Guide](./user-guide/configuration.md) - Updated configuration examples
- [CRD Reference](./user-guide/crd-reference.md) - Complete API reference
- [Saturation Scaling](./saturation-scaling-config.md) - How saturation-based scaling works
- [Prometheus Integration](./integrations/prometheus.md) - Setting up metrics collection
- [HPA Integration](./integrations/hpa-integration.md) - Connecting WVA with HPA

### Getting Help

If you encounter issues during migration:
1. Check the [troubleshooting section](./user-guide/configuration.md#troubleshooting) in the configuration guide
2. Review controller logs for error messages
3. Verify Prometheus metrics are available using `kubectl port-forward`
4. Join the [llm-d community](https://join.slack.com/share/enQtOTg1MzkwODExNDI5Mi02NWQwOWEwOWM4Y2Y3MTc4OTQyY2Y1ZDVlZmU2MjBmZDUwNjJhZGM3MjY4ZTQ5OTdjZjgzMmI0NjI0ZTBhZTM4) for support
