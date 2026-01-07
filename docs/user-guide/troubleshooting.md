# Troubleshooting Guide

This guide helps you diagnose and resolve common issues with Workload-Variant-Autoscaler (WVA).

## Table of Contents

- [Installation Issues](#installation-issues)
- [Controller Issues](#controller-issues)
- [Metrics and Monitoring](#metrics-and-monitoring)
- [Autoscaling Behavior](#autoscaling-behavior)
- [Integration Issues](#integration-issues)
- [Performance Issues](#performance-issues)
- [Getting Help](#getting-help)

## Installation Issues

### CRDs Not Installed

**Symptoms:**
- Error: `no matches for kind "VariantAutoscaling"`
- Controller fails to start

**Solution:**
```bash
# Verify CRDs are installed
kubectl get crd variantautoscalings.llmd.ai

# If missing, install CRDs
kubectl apply -f charts/workload-variant-autoscaler/crds/
```

### Controller Not Starting

**Symptoms:**
- Controller pod in CrashLoopBackOff
- Controller pod not running

**Diagnosis:**
```bash
# Check controller logs
kubectl logs -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager

# Check controller pod status
kubectl describe pod -n workload-variant-autoscaler-system \
  -l control-plane=controller-manager
```

**Common Causes:**

1. **Missing RBAC permissions**
   - Verify ClusterRole and ClusterRoleBinding are created
   - Check controller service account has proper permissions

2. **Prometheus connection issues**
   - Verify Prometheus URL is correct
   - Check TLS certificate configuration
   - Test connection: `kubectl exec -it <controller-pod> -- curl -k <prometheus-url>`

3. **Image pull errors**
   - Verify image repository and tag
   - Check image pull secrets if using private registry

### Helm Installation Failures

**Symptoms:**
- `helm install` or `helm upgrade` fails

**Common Issues:**

1. **CRD already exists**
   ```bash
   # Delete old CRDs first
   kubectl delete crd variantautoscalings.llmd.ai
   
   # Reinstall
   helm install workload-variant-autoscaler ./charts/workload-variant-autoscaler
   ```

2. **Namespace doesn't exist**
   ```bash
   # Create namespace first
   kubectl create namespace workload-variant-autoscaler-system
   ```

3. **Values file errors**
   ```bash
   # Validate values file
   helm template workload-variant-autoscaler ./charts/workload-variant-autoscaler \
     -f custom-values.yaml --debug
   ```

## Controller Issues

### Controller Not Reconciling

**Symptoms:**
- VariantAutoscaling resources not being processed
- Status not updating

**Diagnosis:**
```bash
# Check controller is running
kubectl get pods -n workload-variant-autoscaler-system

# View controller logs
kubectl logs -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager -f

# Check VariantAutoscaling status
kubectl describe variantautoscaling <name> -n <namespace>
```

**Common Causes:**

1. **Target deployment not found**
   - Verify deployment exists and is in the correct namespace
   - Check `scaleTargetRef` or `modelID` field matches actual deployment

2. **Prometheus metrics unavailable**
   - Verify Prometheus is scraping vLLM metrics
   - Check ServiceMonitor is created and configured correctly

### Controller Memory/CPU Issues

**Symptoms:**
- Controller pod being OOMKilled
- High CPU usage

**Solution:**
```yaml
# Increase resource limits in values.yaml
wva:
  resources:
    limits:
      cpu: 1000m
      memory: 1Gi
    requests:
      cpu: 200m
      memory: 256Mi
```

## Metrics and Monitoring

### Metrics Not Appearing in Prometheus

**Symptoms:**
- WVA metrics not visible in Prometheus
- HPA shows `<unknown>` for target values

**Diagnosis:**
```bash
# Check ServiceMonitor exists
kubectl get servicemonitor -n workload-variant-autoscaler-system

# Check Prometheus targets
kubectl port-forward -n workload-variant-autoscaler-monitoring \
  svc/kube-prometheus-stack-prometheus 9090:9090
# Visit http://localhost:9090/targets

# Test metrics endpoint directly
kubectl port-forward -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager 8443:8443
# Visit https://localhost:8443/metrics (accept self-signed cert)
```

**Common Causes:**

1. **ServiceMonitor not created**
   ```bash
   kubectl apply -f charts/workload-variant-autoscaler/templates/manager/wva-servicemonitor.yaml
   ```

2. **Prometheus doesn't have RBAC to scrape metrics**
   ```bash
   # Verify RBAC exists
   kubectl get clusterrolebinding prometheus-wva-metrics
   ```

3. **Network policies blocking traffic**
   - Check network policies allow traffic from Prometheus namespace to WVA namespace
   - Verify firewall rules if applicable

### vLLM Metrics Not Available

**Symptoms:**
- WVA can't get vLLM server metrics
- `inferno_vllm_*` metrics missing

**Solution:**
```bash
# Verify vLLM ServiceMonitor exists
kubectl get servicemonitor -n <vllm-namespace>

# Check vLLM metrics endpoint
kubectl port-forward -n <vllm-namespace> svc/<vllm-service> 8000:8000
curl http://localhost:8000/metrics

# Verify Prometheus is scraping vLLM
# Check Prometheus targets (see above)
```

## Autoscaling Behavior

### HPA Not Scaling

**Symptoms:**
- Deployment not scaling despite metrics showing different desired replicas

**Diagnosis:**
```bash
# Check HPA status
kubectl get hpa -n <namespace>
kubectl describe hpa <hpa-name> -n <namespace>

# Check external metrics
kubectl get --raw "/apis/external.metrics.k8s.io/v1beta1/namespaces/<namespace>/inferno_desired_replicas"
```

**Common Causes:**

1. **Prometheus Adapter not installed or misconfigured**
   ```bash
   # Verify Prometheus Adapter is running
   kubectl get pods -n workload-variant-autoscaler-monitoring | grep adapter
   
   # Check adapter logs
   kubectl logs -n workload-variant-autoscaler-monitoring \
     deployment/prometheus-adapter
   ```

2. **Metric query returns no data**
   - Check VariantAutoscaling resource has correct labels
   - Verify metric name matches HPA configuration

3. **HPA stabilization window too long**
   - Reduce `stabilizationWindowSeconds` in HPA spec
   - See [HPA Integration Guide](../integrations/hpa-integration.md)

### Scaling Too Slow

**Symptoms:**
- Autoscaling takes too long to respond to load changes

**Solutions:**

1. **Reduce reconciliation interval**
   ```yaml
   wva:
     reconcileInterval: "30s"  # Default: 60s
   ```

2. **Adjust HPA stabilization window**
   ```yaml
   behavior:
     scaleUp:
       stabilizationWindowSeconds: 60  # Default: 240
     scaleDown:
       stabilizationWindowSeconds: 180  # Default: 240
   ```

3. **Increase HPA check interval**
   - Configure kube-controller-manager `--horizontal-pod-autoscaler-sync-period` (default: 15s)

### Scaling Too Aggressive

**Symptoms:**
- Frequent scale up/down events
- Unstable replica count

**Solutions:**

1. **Increase stabilization windows**
   ```yaml
   behavior:
     scaleUp:
       stabilizationWindowSeconds: 300
     scaleDown:
       stabilizationWindowSeconds: 600
   ```

2. **Add scale-down/up policies**
   ```yaml
   behavior:
     scaleDown:
       policies:
       - type: Percent
         value: 10
         periodSeconds: 60
       - type: Pods
         value: 2
         periodSeconds: 120
   ```

### Deployment Not Being Targeted

**Symptoms:**
- VariantAutoscaling resource created but doesn't affect any deployment

**Diagnosis:**
```bash
# Check VariantAutoscaling status
kubectl get variantautoscaling <name> -n <namespace> -o yaml

# Look for conditions and status fields
```

**Solutions:**

1. **Specify scaleTargetRef explicitly**
   ```yaml
   spec:
     modelId: "meta/llama-3.1-8b"
     scaleTargetRef:
       name: my-vllm-deployment
       kind: Deployment
   ```

2. **Ensure deployment has correct labels**
   - Check deployment has labels that match WVA's discovery mechanism

## Integration Issues

### KEDA Integration Problems

See [KEDA Integration Guide](../integrations/keda-integration.md) for detailed troubleshooting.

**Quick checks:**
```bash
# Verify KEDA is installed
kubectl get pods -n keda

# Check ScaledObject status
kubectl describe scaledobject <name> -n <namespace>
```

### Prometheus Adapter Issues

**Symptoms:**
- External metrics API not responding
- HPA can't read metrics

**Solution:**
```bash
# Check adapter pods
kubectl get pods -n workload-variant-autoscaler-monitoring | grep adapter

# Test external metrics API
kubectl get --raw "/apis/external.metrics.k8s.io/v1beta1"

# Check adapter configuration
kubectl get configmap prometheus-adapter -n workload-variant-autoscaler-monitoring -o yaml
```

## Performance Issues

### High Reconciliation Latency

**Symptoms:**
- Controller logs show slow reconciliation times

**Solutions:**

1. **Optimize Prometheus queries**
   - Use recording rules for complex queries
   - Increase Prometheus query timeout

2. **Adjust reconciliation interval**
   ```yaml
   wva:
     reconcileInterval: "120s"  # Reduce frequency
   ```

3. **Scale controller vertically**
   - Increase CPU/memory limits

### Prometheus Query Timeouts

**Symptoms:**
- Controller logs show `context deadline exceeded`
- Metrics collection failures

**Solutions:**

1. **Increase query timeout**
   ```yaml
   wva:
     prometheus:
       queryTimeout: "60s"  # Default: 30s
   ```

2. **Use Prometheus recording rules**
   - Pre-compute expensive queries
   - Reduce query complexity

## Debugging Techniques

### Enable Debug Logging

```bash
# Redeploy with debug logging
helm upgrade workload-variant-autoscaler ./charts/workload-variant-autoscaler \
  --set wva.logLevel=debug \
  -n workload-variant-autoscaler-system
```

### Port Forward for Local Testing

```bash
# Access Prometheus locally
kubectl port-forward -n workload-variant-autoscaler-monitoring \
  svc/kube-prometheus-stack-prometheus 9090:9090

# Access controller metrics
kubectl port-forward -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager 8443:8443

# Access vLLM metrics
kubectl port-forward -n <vllm-namespace> \
  deployment/<vllm-deployment> 8000:8000
```

### Check Resource Events

```bash
# Watch VariantAutoscaling events
kubectl get events -n <namespace> --field-selector involvedObject.kind=VariantAutoscaling

# Watch HPA events
kubectl get events -n <namespace> --field-selector involvedObject.kind=HorizontalPodAutoscaler

# Watch Deployment events
kubectl get events -n <namespace> --field-selector involvedObject.kind=Deployment
```

## Getting Help

If you're still experiencing issues after trying these troubleshooting steps:

1. **Check existing issues**: [GitHub Issues](https://github.com/llm-d-incubation/workload-variant-autoscaler/issues)

2. **Open a new issue** with:
   - WVA version
   - Kubernetes/OpenShift version
   - Complete error logs
   - Steps to reproduce
   - VariantAutoscaling resource YAML
   - Deployment YAML

3. **Join community discussions**:
   - [Community meetings](https://github.com/llm-d-incubation/workload-variant-autoscaler)
   - Slack channel (link in README)

4. **Review additional resources**:
   - [Developer Guide](../developer-guide/development.md)
   - [Debugging Guide](../developer-guide/debugging.md)
   - [Architecture Documentation](../design/modeling-optimization.md)

## See Also

- [Installation Guide](installation.md)
- [Configuration Guide](configuration.md)
- [HPA Integration](../integrations/hpa-integration.md)
- [KEDA Integration](../integrations/keda-integration.md)
- [Metrics Guide](../metrics-health-monitoring.md)
