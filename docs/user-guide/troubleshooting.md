# Troubleshooting Guide

This guide helps diagnose and resolve common issues with Workload-Variant-Autoscaler (WVA).

## Table of Contents

- [Diagnostic Tools](#diagnostic-tools)
- [Common Issues](#common-issues)
- [Controller Issues](#controller-issues)
- [Metrics & Monitoring Issues](#metrics--monitoring-issues)
- [Scaling Issues](#scaling-issues)
- [Integration Issues](#integration-issues)
- [Performance Issues](#performance-issues)
- [Getting Help](#getting-help)

## Diagnostic Tools

### Check VariantAutoscaling Resource Status

```bash
# View detailed status
kubectl get variantautoscaling <name> -n <namespace> -o yaml

# Check conditions
kubectl get variantautoscaling <name> -n <namespace> -o jsonpath='{.status.conditions}' | jq
```

Key status fields:
- `status.conditions`: Current state and error messages
- `status.desiredOptimizedAlloc`: Target allocation from WVA
- `status.actuation`: Actuation details and timestamps

### Check Controller Logs

```bash
# View recent logs
kubectl logs -n workload-variant-autoscaler-system deployment/workload-variant-autoscaler-controller-manager --tail=100

# Follow logs
kubectl logs -n workload-variant-autoscaler-system deployment/workload-variant-autoscaler-controller-manager -f

# Filter for specific resource
kubectl logs -n workload-variant-autoscaler-system deployment/workload-variant-autoscaler-controller-manager | grep "variantautoscaling=<name>"
```

### Verify Prerequisites

```bash
# Check WVA controller is running
kubectl get deployment -n workload-variant-autoscaler-system

# Check Prometheus connectivity
kubectl get svc -n monitoring prometheus-k8s  # Adjust namespace as needed

# Verify target deployment exists
kubectl get deployment <deployment-name> -n <namespace>

# Check HPA/KEDA
kubectl get hpa -n <namespace>
kubectl get scaledobject -n <namespace>  # For KEDA
```

### Check Metrics Availability

```bash
# Test Prometheus connectivity from WVA controller pod
kubectl exec -n workload-variant-autoscaler-system deployment/workload-variant-autoscaler-controller-manager -- curl -s http://prometheus-k8s.monitoring.svc:9090/api/v1/query?query=up

# Check if vLLM metrics are being scraped
kubectl exec -n workload-variant-autoscaler-system deployment/workload-variant-autoscaler-controller-manager -- curl -s "http://prometheus-k8s.monitoring.svc:9090/api/v1/query?query=vllm_request_prompt_tokens"
```

## Common Issues

### VariantAutoscaling Resource Not Reconciling

**Symptoms**: Status shows stale data or no updates

**Diagnosis**:

```bash
# Check controller logs for reconciliation errors
kubectl logs -n workload-variant-autoscaler-system deployment/workload-variant-autoscaler-controller-manager | grep ERROR

# Verify resource is not paused
kubectl get variantautoscaling <name> -n <namespace> -o jsonpath='{.metadata.annotations}' | grep paused
```

**Solutions**:

1. **Check controller is running**:
   ```bash
   kubectl get pods -n workload-variant-autoscaler-system
   ```

2. **Restart controller**:
   ```bash
   kubectl rollout restart deployment/workload-variant-autoscaler-controller-manager -n workload-variant-autoscaler-system
   ```

3. **Verify RBAC permissions**:
   ```bash
   kubectl auth can-i get deployments --as=system:serviceaccount:workload-variant-autoscaler-system:workload-variant-autoscaler-controller-manager -n <namespace>
   ```

### Target Deployment Not Found

**Symptoms**: Condition shows "DeploymentNotFound" or similar error

**Diagnosis**:

```bash
# Check if deployment exists
kubectl get deployment <deployment-name> -n <namespace>

# Verify scaleTargetRef in VariantAutoscaling spec
kubectl get variantautoscaling <name> -n <namespace> -o jsonpath='{.spec.scaleTargetRef}'
```

**Solutions**:

1. **Create the deployment** if it doesn't exist
2. **Fix scaleTargetRef** if it points to the wrong resource:
   ```bash
   kubectl patch variantautoscaling <name> -n <namespace> --type=merge -p '{"spec":{"scaleTargetRef":{"kind":"Deployment","name":"correct-name"}}}'
   ```
3. **Check namespace**: Ensure VariantAutoscaling and Deployment are in the same namespace

## Controller Issues

### Controller Pod CrashLooping

**Diagnosis**:

```bash
# Check pod status
kubectl get pods -n workload-variant-autoscaler-system

# View recent logs
kubectl logs -n workload-variant-autoscaler-system deployment/workload-variant-autoscaler-controller-manager --previous
```

**Common Causes**:

1. **Invalid ConfigMap**: Check ConfigMap syntax
   ```bash
   kubectl get configmap -n workload-variant-autoscaler-system
   kubectl get configmap workload-variant-autoscaler-config -n workload-variant-autoscaler-system -o yaml
   ```

2. **Prometheus connection issues**: Verify Prometheus endpoint
3. **Certificate issues**: Check TLS certificates if using secure Prometheus

### High Memory Usage

**Diagnosis**:

```bash
# Check resource usage
kubectl top pod -n workload-variant-autoscaler-system

# View memory limits
kubectl get deployment workload-variant-autoscaler-controller-manager -n workload-variant-autoscaler-system -o jsonpath='{.spec.template.spec.containers[0].resources}'
```

**Solutions**:

1. **Increase memory limits** via Helm values:
   ```yaml
   resources:
     limits:
       memory: 512Mi
     requests:
       memory: 256Mi
   ```

2. **Reduce polling frequency** in ConfigMap
3. **Check for metrics leaks** in controller logs

## Metrics & Monitoring Issues

### Prometheus Metrics Not Available

**Symptoms**: WVA status shows "MetricsUnavailable" condition

**Diagnosis**:

```bash
# Test Prometheus connectivity
kubectl run -it --rm curl --image=curlimages/curl --restart=Never -- \
  curl -s http://prometheus-k8s.monitoring.svc:9090/api/v1/query?query=up

# Check Prometheus is scraping vLLM pods
kubectl get servicemonitor -n <vllm-namespace>
```

**Solutions**:

1. **Verify Prometheus configuration**:
   - Ensure ServiceMonitor exists for vLLM pods
   - Check Prometheus service discovery configuration
   - Verify network connectivity between Prometheus and vLLM pods

2. **Update Prometheus endpoint in WVA ConfigMap**:
   ```yaml
   prometheus:
     endpoint: "http://prometheus-k8s.monitoring.svc:9090"
   ```

3. **Check TLS configuration** if using secure Prometheus:
   ```bash
   kubectl get secret -n workload-variant-autoscaler-system
   ```

### HPA Not Reading WVA Metrics

**Symptoms**: HPA shows "unknown" for custom metrics

**Diagnosis**:

```bash
# Check HPA status
kubectl get hpa <hpa-name> -n <namespace> -o yaml

# Test custom metrics API
kubectl get --raw "/apis/custom.metrics.k8s.io/v1beta1/namespaces/<namespace>/pods/*/wva_desired_replicas" | jq
```

**Solutions**:

1. **Install Prometheus Adapter** if not present:
   ```bash
   helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
   helm install prometheus-adapter prometheus-community/prometheus-adapter
   ```

2. **Configure Prometheus Adapter** to expose WVA metrics. See [HPA Integration](../integrations/hpa-integration.md) for configuration.

3. **Verify WVA is emitting metrics**:
   ```bash
   kubectl exec -n workload-variant-autoscaler-system deployment/workload-variant-autoscaler-controller-manager -- curl -s localhost:8080/metrics | grep wva_desired_replicas
   ```

### vLLM Metrics Not Scraped

**Diagnosis**:

```bash
# Check vLLM ServiceMonitor
kubectl get servicemonitor -n <namespace>

# Verify vLLM service has correct labels
kubectl get svc <vllm-service> -n <namespace> --show-labels

# Check Prometheus targets
kubectl port-forward -n monitoring svc/prometheus-k8s 9090:9090
# Then visit http://localhost:9090/targets in browser
```

**Solutions**:

1. **Create ServiceMonitor** for vLLM pods. See [Prometheus Integration](../integrations/prometheus.md).

2. **Verify vLLM metrics endpoint**:
   ```bash
   kubectl port-forward -n <namespace> <vllm-pod> 8000:8000
   curl http://localhost:8000/metrics
   ```

3. **Check Prometheus RBAC** permissions to discover vLLM pods

## Scaling Issues

### Replicas Not Scaling

**Symptoms**: Desired replicas calculated by WVA, but deployment not scaling

**Diagnosis**:

```bash
# Check HPA is reading metrics
kubectl get hpa <hpa-name> -n <namespace> -o yaml

# Verify current vs desired replicas
kubectl get deployment <deployment-name> -n <namespace>

# Check VariantAutoscaling status
kubectl get variantautoscaling <name> -n <namespace> -o jsonpath='{.status.desiredOptimizedAlloc}'
```

**Solutions**:

1. **Verify HPA configuration** matches WVA metric name:
   ```yaml
   metrics:
   - type: Pods
     pods:
       metric:
         name: wva_desired_replicas
       target:
         type: AverageValue
         averageValue: "1"
   ```

2. **Check HPA stabilization window**: Ensure it's not preventing scaling:
   ```yaml
   behavior:
     scaleDown:
       stabilizationWindowSeconds: 300
   ```

3. **Check deployment minReplicas/maxReplicas** in HPA

### Scaling Too Aggressively

**Symptoms**: Replicas fluctuate rapidly

**Solutions**:

1. **Increase HPA stabilization window**:
   ```yaml
   behavior:
     scaleUp:
       stabilizationWindowSeconds: 120
     scaleDown:
       stabilizationWindowSeconds: 300
   ```

2. **Adjust saturation thresholds** in WVA ConfigMap. See [Saturation Scaling Configuration](../saturation-scaling-config.md).

3. **Increase HPA metric polling interval**

### Scale to Zero Not Working

**Diagnosis**:

```bash
# Check if scale-to-zero is configured
kubectl get variantautoscaling <name> -n <namespace> -o yaml | grep -A 5 scaleFromZero

# Verify KEDA is installed (if using KEDA for scale-to-zero)
kubectl get scaledobject -n <namespace>
```

**Solutions**:

1. **Enable scale-to-zero** in VariantAutoscaling spec (if supported)
2. **Use KEDA** instead of HPA for scale-to-zero scenarios. See [KEDA Integration](../integrations/keda-integration.md).
3. **Configure minimum replicas** appropriately

## Integration Issues

### HPA Integration Not Working

See [HPA Integration Troubleshooting](../integrations/hpa-integration.md#troubleshooting)

### KEDA Integration Not Working

See [KEDA Integration Troubleshooting](../integrations/keda-integration.md#troubleshooting)

### OpenShift-Specific Issues

**Diagnosis**:

```bash
# Check SecurityContextConstraints
oc get scc -o name | xargs -I {} oc describe {} | grep workload-variant-autoscaler

# Verify ServiceAccount permissions
oc auth can-i get deployments --as=system:serviceaccount:workload-variant-autoscaler-system:workload-variant-autoscaler-controller-manager
```

**Solutions**:

1. **Apply OpenShift-specific configuration**:
   ```bash
   kubectl apply -k config/openshift
   ```

2. **Configure monitoring permissions** for OpenShift Monitoring. See [OpenShift Deployment](../../deploy/openshift/README.md).

3. **Use OpenShift routes** instead of Ingress

## Performance Issues

### High Latency in Reconciliation

**Diagnosis**:

```bash
# Check reconciliation metrics
kubectl exec -n workload-variant-autoscaler-system deployment/workload-variant-autoscaler-controller-manager -- curl -s localhost:8080/metrics | grep reconcile_duration

# View controller logs for slow operations
kubectl logs -n workload-variant-autoscaler-system deployment/workload-variant-autoscaler-controller-manager | grep "took longer"
```

**Solutions**:

1. **Reduce Prometheus query complexity** - optimize queries in ConfigMap
2. **Increase controller resources**
3. **Tune polling intervals** to reduce query frequency
4. **Check Prometheus performance** - ensure it can handle query load

### Controller Using Too Many Resources

**Solutions**:

1. **Reduce number of managed VariantAutoscaling resources**
2. **Increase reconciliation interval**
3. **Optimize metric queries** to return less data
4. **Enable metric caching** if available

## Getting Help

### Collect Diagnostic Information

Before seeking help, collect:

1. **Controller logs**:
   ```bash
   kubectl logs -n workload-variant-autoscaler-system deployment/workload-variant-autoscaler-controller-manager --tail=500 > wva-controller.log
   ```

2. **VariantAutoscaling resource**:
   ```bash
   kubectl get variantautoscaling <name> -n <namespace> -o yaml > variantautoscaling.yaml
   ```

3. **Controller configuration**:
   ```bash
   kubectl get configmap -n workload-variant-autoscaler-system -o yaml > configmaps.yaml
   ```

4. **HPA/KEDA configuration**:
   ```bash
   kubectl get hpa,scaledobject -n <namespace> -o yaml > autoscalers.yaml
   ```

5. **Environment information**:
   ```bash
   kubectl version
   kubectl get nodes
   ```

### Open an Issue

When opening a [GitHub issue](https://github.com/llm-d-incubation/workload-variant-autoscaler/issues):

1. **Describe the problem** clearly
2. **Attach diagnostic information** (sanitize sensitive data)
3. **Include reproduction steps** if possible
4. **Specify your environment** (Kubernetes version, OpenShift, cloud provider)

### Additional Resources

- [FAQ](faq.md)
- [Configuration Guide](configuration.md)
- [Developer Debugging Guide](../developer-guide/debugging.md)
- [HPA Integration](../integrations/hpa-integration.md)
- [KEDA Integration](../integrations/keda-integration.md)

---

**Still stuck?** Open an issue or join community meetings - we're here to help!
