# Frequently Asked Questions (FAQ)

This document answers common questions about the Workload-Variant-Autoscaler (WVA).

## General Questions

### What is WVA?

The Workload-Variant-Autoscaler (WVA) is a Kubernetes controller that performs intelligent autoscaling for inference model servers based on saturation metrics. It analyzes KV cache utilization and queue depth to determine optimal replica counts, ensuring efficient resource usage while meeting performance requirements.

### How does WVA differ from HPA or KEDA?

WVA complements HPA and KEDA rather than replacing them:

- **WVA** analyzes model server saturation and computes optimal replica counts, emitting custom metrics
- **HPA/KEDA** reads these metrics and performs the actual scaling operations
- WVA provides workload-aware autoscaling optimized for LLM inference patterns

See [HPA Integration](../integrations/hpa-integration.md) and [KEDA Integration](../integrations/keda-integration.md) for details.

### What model servers does WVA support?

Currently, WVA is designed for and tested with **vLLM** servers. The controller monitors vLLM-specific metrics including:
- KV cache utilization
- Request queue depth
- GPU memory usage

Support for additional inference servers may be added in the future.

### Do I need physical GPUs to try WVA?

No! WVA includes a complete emulation environment for local development:

```bash
make deploy-llm-d-wva-emulated-on-kind
```

This creates a Kind cluster with emulated GPUs and works on Mac (Apple Silicon/Intel) and Windows. See [Local Development](../deploy/kind-emulator/README.md).

## Installation & Configuration

### Can I run multiple WVA controllers in the same cluster?

Yes! WVA supports multi-controller isolation using label selectors. This allows you to:
- Run separate controller instances for different teams or environments
- Configure different scaling policies per controller
- Isolate controller scopes to specific namespaces

See [Multi-Controller Isolation](multi-controller-isolation.md) for configuration details.

### What Kubernetes versions are supported?

- **Kubernetes**: v1.31.0 or later
- **OpenShift**: 4.18 or later

### How do I configure Prometheus for WVA?

WVA requires Prometheus to collect vLLM metrics. Configuration depends on your deployment:

- **Kubernetes**: Use Prometheus Operator or the provided all-in-one deployment
- **OpenShift**: Use the built-in Prometheus or configure an external instance

See [Installation Guide](installation.md) and [Prometheus Integration](../integrations/prometheus.md).

### What happens if I create a VariantAutoscaling CR before the deployment exists?

WVA handles this gracefully:
- The controller will update the VA status to indicate the deployment is missing
- Once the deployment is created, WVA automatically resumes operation
- No manual intervention is required

### Should I create the VariantAutoscaling before or after the deployment?

Either order works! However, the recommended approach is:
1. Deploy your model server and wait for it to warm up
2. Create the VariantAutoscaling CR

This prevents immediate scale-down before your deployment is ready to serve traffic.

## Scaling Behavior

### How does WVA determine the desired replica count?

WVA uses a **saturation-based capacity model**:
1. Collects KV cache utilization and queue depth from Prometheus
2. Analyzes saturation levels across all replicas
3. Identifies replicas with slack capacity
4. Computes optimal replica count based on configurable thresholds
5. Emits the desired count as a custom metric for HPA/KEDA

See [Saturation Scaling Configuration](../saturation-scaling-config.md) for algorithm details.

### Why isn't my deployment scaling?

Check these common issues:

1. **HPA/KEDA not configured**: WVA emits metrics but doesn't scale directly
   - Verify HPA or KEDA resource exists: `kubectl get hpa -n <namespace>`
   - Check HPA status: `kubectl describe hpa <name> -n <namespace>`

2. **Prometheus not collecting metrics**: 
   - Verify ServiceMonitor exists: `kubectl get servicemonitor -n <namespace>`
   - Check Prometheus targets: Access Prometheus UI and verify vLLM targets are UP

3. **VariantAutoscaling status shows errors**:
   - Check VA status: `kubectl describe variantautoscaling <name> -n <namespace>`
   - Review controller logs: `kubectl logs -n workload-variant-autoscaler-system deployment/workload-variant-autoscaler-controller-manager`

4. **HPA stabilization window preventing scale**:
   - HPA may be in cooldown period
   - Check `scaleDownStabilizationWindowSeconds` in HPA spec (recommended: 120s+)

See [Troubleshooting Guide](troubleshooting.md) for detailed diagnostics.

### How fast does WVA respond to traffic changes?

Response time depends on several factors:
- **WVA reconciliation interval**: Default 60 seconds (configurable)
- **Prometheus scrape interval**: Typically 15-30 seconds
- **HPA sync period**: Default 15 seconds
- **HPA stabilization window**: Recommended 120+ seconds for gradual scaling

Total end-to-end latency is typically 2-5 minutes for scale decisions.

### Can WVA scale to zero?

Yes, but with limitations:
- WVA includes a scale-from-zero engine
- Requires at least one replica to collect metrics
- Zero-replica scaling may require KEDA with appropriate triggers

See the scale-from-zero documentation (coming soon) for configuration details.

### What is the variantCost field used for?

The `variantCost` field is used in the saturation analysis to determine scaling priorities. It represents the cost per replica and helps WVA make cost-aware scaling decisions. Default value is "10.0".

## Metrics & Monitoring

### What metrics does WVA emit?

WVA emits several custom metrics to Prometheus:

- `wva_desired_replicas`: Optimal replica count computed by WVA
- `wva_current_replicas`: Current replica count
- `wva_saturation_level`: Model server saturation metrics
- Controller performance metrics (reconciliation duration, errors, etc.)

See [Prometheus Metrics](../integrations/prometheus.md) for the complete list.

### How can I monitor WVA health?

Monitor these key indicators:

1. **Controller health**: Check pod status and logs
2. **Reconciliation success rate**: View controller metrics in Prometheus
3. **Scaling effectiveness**: Compare desired vs. current replicas
4. **vLLM metrics**: Monitor KV cache utilization and queue depth

See [Metrics & Health Monitoring](../metrics-health-monitoring.md).

## Architecture & Limitations

### What model architectures does WVA support?

WVA is designed for standard transformer-based LLMs. **Important limitations**:

- **Not supported**: Hybrid state-space models (HSSM), Mixture of Experts (MoE), or non-standard architectures
- These architectures have different performance characteristics that may not align with WVA's capacity model

See [Architecture Limitations](../design/architecture-limitations.md) for critical information.

### How does WVA handle multi-model deployments?

Each model deployment should have its own VariantAutoscaling CR. WVA controllers operate independently per model variant, allowing:
- Different scaling policies per model
- Independent replica management
- Model-specific cost optimization

### Can I use custom accelerators?

Yes! WVA supports configurable accelerator types. You can:
- Define accelerator types in the ConfigMap
- Specify per-accelerator unit costs
- Configure model profiles with specific accelerator requirements

See [Configuration Guide](configuration.md) for accelerator setup.

## Upgrading & Migration

### How do I upgrade WVA?

**Important**: Helm does not automatically update CRDs. Follow this process:

```bash
# 1. Apply updated CRDs manually
kubectl apply -f charts/workload-variant-autoscaler/crds/

# 2. Upgrade Helm release
helm upgrade workload-variant-autoscaler ./charts/workload-variant-autoscaler \
  --namespace workload-variant-autoscaler-system \
  [your-values...]
```

See [Upgrading](installation.md#upgrading) for version-specific notes.

### Are there breaking changes I should know about?

Check the [README.md](../../README.md#breaking-changes) for version-specific breaking changes. Notable upcoming changes:

- **v0.5.0**: Added `scaleTargetRef` field to VariantAutoscaling CRD (with backward compatibility)

## Troubleshooting

### Where are the WVA controller logs?

```bash
kubectl logs -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager \
  --tail=100 --follow
```

### How do I enable debug logging?

Set the log level in the controller deployment:

```bash
helm upgrade workload-variant-autoscaler ./charts/workload-variant-autoscaler \
  --set controller.logLevel=debug \
  --reuse-values
```

### WVA status shows "DeploymentNotFound"

This indicates the target deployment doesn't exist or isn't accessible:

1. Verify the deployment exists: `kubectl get deployment <name> -n <namespace>`
2. Check the `scaleTargetRef` in your VariantAutoscaling CR
3. Ensure WVA has RBAC permissions to access the deployment

### Metrics aren't updating

Check the Prometheus integration:

1. Verify ServiceMonitor exists: `kubectl get servicemonitor -n <namespace>`
2. Check Prometheus scraping: Access Prometheus UI → Targets
3. Verify vLLM endpoints are accessible: `kubectl get svc -n <namespace>`
4. Review WVA controller logs for collection errors

## Getting Help

### Where can I find more information?

- **Documentation**: [docs/](../README.md)
- **Examples**: [config/samples/](../../config/samples/)
- **Deployment guides**: [deploy/](../../deploy/README.md)

### How do I report bugs or request features?

- **GitHub Issues**: [Open an issue](https://github.com/llm-d-incubation/workload-variant-autoscaler/issues)
- **Community meetings**: Join the [llm-d autoscaling community meetings](https://join.slack.com/share/enQtOTg1MzkwODExNDI5Mi02NWQwOWEwOWM4Y2Y3MTc4OTQyY2Y1ZDVlZmU2MjBmZDUwNjJhZGM3MjY4ZTQ5OTdjZjgzMmI0NjI0ZTBhZTM4)

### How can I contribute?

We welcome contributions! See [CONTRIBUTING.md](../../CONTRIBUTING.md) and [Developer Guide](../developer-guide/development.md) to get started.

---

**Didn't find your answer?** Open an issue or ask in the community meetings!
