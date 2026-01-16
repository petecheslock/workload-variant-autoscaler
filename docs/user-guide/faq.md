# Frequently Asked Questions (FAQ)

This document answers common questions about Workload-Variant-Autoscaler (WVA).

## General Questions

### What is WVA?

Workload-Variant-Autoscaler (WVA) is a Kubernetes controller that performs intelligent autoscaling for inference model servers based on saturation metrics. It optimizes replica counts for inference workloads to balance cost and performance.

### How does WVA differ from HPA or KEDA?

WVA complements rather than replaces HPA and KEDA. WVA analyzes saturation metrics (KV cache utilization, queue depth) and emits custom metrics that HPA or KEDA consume to make scaling decisions. Think of WVA as the "brain" that determines optimal replica counts, while HPA/KEDA handle the actual scaling.

See [HPA Integration](../integrations/hpa-integration.md) and [KEDA Integration](../integrations/keda-integration.md) for details.

### Which Kubernetes versions are supported?

WVA requires:
- Kubernetes v1.31.0+ 
- OpenShift 4.18+ (for OpenShift deployments)

### Which model servers does WVA support?

WVA currently supports vLLM servers that expose Prometheus metrics for:
- KV cache utilization
- Queue depth
- Request counts

Other inference servers can be supported if they expose compatible metrics.

## Installation & Setup

### Can I test WVA without GPUs?

Yes! WVA includes a Kind-based emulator that works on Mac (Apple Silicon/Intel) and Windows without physical GPUs:

```bash
make deploy-wva-emulated-on-kind CREATE_CLUSTER=true DEPLOY_LLM_D=true
```

See [Local Development with Kind Emulator](../../deploy/kind-emulator/README.md) for details.

### How do I install WVA on my cluster?

See the [Installation Guide](installation.md) for Helm-based installation and the deployment READMEs:
- [Kubernetes Deployment](../../deploy/kubernetes/README.md)
- [OpenShift Deployment](../../deploy/openshift/README.md)

### Do I need Prometheus?

Yes, WVA requires Prometheus for:
1. Collecting metrics from vLLM servers
2. Exposing optimization metrics for HPA/KEDA consumption

See [Prometheus Integration](../integrations/prometheus.md) for setup details.

## Configuration

### What does `variantCost` mean in the VariantAutoscaling CR?

`variantCost` represents the relative cost per replica for a model variant. It defaults to "10.0" and is used in saturation analysis. Higher values indicate more expensive replicas (e.g., models requiring larger GPUs).

### When should I create the VariantAutoscaling CR?

WVA handles the creation order gracefully:
- You can create the VariantAutoscaling CR before or after the deployment
- If a deployment is deleted, the VA status updates to reflect the missing deployment
- When the deployment is recreated, the VA automatically resumes operation

### How do I configure the HPA stabilization window?

Configure HPA with a stabilization window of 120 seconds or more for gradual scaling:

```yaml
behavior:
  scaleUp:
    stabilizationWindowSeconds: 120
  scaleDown:
    stabilizationWindowSeconds: 300
```

See [HPA Integration](../integrations/hpa-integration.md) for complete examples.

### Can I run multiple WVA controllers?

Yes! WVA supports multi-controller isolation for different tenants or workload types. See [Multi-Controller Isolation](multi-controller-isolation.md) for details.

## Operation

### How do I monitor WVA?

WVA exposes Prometheus metrics for monitoring:
- Optimization metrics (desired replicas, allocations)
- Controller health metrics
- Reconciliation performance

See [Metrics & Health Monitoring](../metrics-health-monitoring.md) for details.

### What happens if metrics are unavailable?

If Prometheus or vLLM metrics are unavailable, WVA:
1. Updates the VariantAutoscaling status conditions to indicate the issue
2. Maintains the last known good state
3. Resumes normal operation when metrics become available again

### How often does WVA recalculate scaling decisions?

WVA reconciles VariantAutoscaling resources based on:
- Deployment changes (every 10 minutes by default)
- Metrics updates (configurable polling interval)
- Manual triggering via annotation updates

The reconciliation interval is configurable via controller flags.

## Troubleshooting

### WVA is not scaling my deployment. What should I check?

1. **Verify VariantAutoscaling status**: `kubectl get variantautoscaling <name> -o yaml`
2. **Check conditions**: Look for error conditions in the status
3. **Verify HPA/KEDA**: Ensure your HPA/KEDA is reading WVA's custom metrics
4. **Check Prometheus connectivity**: Verify WVA can reach Prometheus
5. **Review logs**: `kubectl logs -n workload-variant-autoscaler-system deployment/workload-variant-autoscaler-controller-manager`

See [Troubleshooting Guide](troubleshooting.md) for detailed diagnostics.

### How do I debug WVA controller issues?

See the [Debugging Guide](../developer-guide/debugging.md) for:
- Remote debugging with dlv
- SSH tunnel setup for remote clusters
- Log analysis techniques

### I'm getting "unknown model architecture" warnings

Some model architectures (HSSM, MoE, custom architectures) have specific requirements. See [Architecture Limitations](../design/architecture-limitations.md) for details.

## Advanced Topics

### Can WVA scale to zero?

Scale-to-zero capabilities depend on your configuration. WVA can work with scale-to-zero patterns when integrated with appropriate event-driven triggers. See the configuration documentation for details.

### Does WVA support multi-model deployments?

Yes, create separate VariantAutoscaling resources for each model deployment. WVA manages each independently while optimizing global resource allocation.

### Can I customize the saturation threshold?

Yes, saturation thresholds are configurable via ConfigMap. See [Saturation Scaling Configuration](../saturation-scaling-config.md) for tuning parameters.

## Contributing

### How can I contribute to WVA?

See [Contributing Guide](../../CONTRIBUTING.md) and [Developer Guide](../developer-guide/development.md) for:
- Development setup
- Testing requirements
- PR process
- Community standards

### Where can I get help?

- Open a [GitHub Issue](https://github.com/llm-d-incubation/workload-variant-autoscaler/issues)
- Check existing documentation
- Join community meetings (see README)

## Additional Resources

- [Configuration Guide](configuration.md)
- [CRD Reference](crd-reference.md)
- [Design & Architecture](../design/modeling-optimization.md)
- [Tutorials](../tutorials/demo.md)

---

**Have a question not answered here?** Please [open an issue](https://github.com/llm-d-incubation/workload-variant-autoscaler/issues) so we can add it to this FAQ!
