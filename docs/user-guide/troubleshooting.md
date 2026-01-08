# Troubleshooting Guide

This guide helps you diagnose and resolve common issues with the Workload-Variant-Autoscaler (WVA).

## Quick Diagnostics

Run these commands to quickly check WVA health:

```bash
# Check controller status
kubectl get pods -n workload-variant-autoscaler-system

# Check VariantAutoscaling resources
kubectl get variantautoscaling -A

# Check controller logs
kubectl logs -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager \
  --tail=50
```

## Common Issues

### Controller Pod Not Running

**Symptoms**: Controller pod is in `CrashLoopBackOff`, `ImagePullBackOff`, or `Error` state.

**Diagnosis**:
```bash
# Check pod status
kubectl get pods -n workload-variant-autoscaler-system

# Describe pod for events
kubectl describe pod -n workload-variant-autoscaler-system \
  -l control-plane=controller-manager

# Check logs
kubectl logs -n workload-variant-autoscaler-system \
  -l control-plane=controller-manager --previous
```

**Solutions**:

1. **ImagePullBackOff**: Verify image name and pull secrets
   ```bash
   kubectl get deployment -n workload-variant-autoscaler-system \
     workload-variant-autoscaler-controller-manager -o yaml | grep image:
   ```

2. **CrashLoopBackOff**: Check logs for startup errors
   - Common causes: Invalid ConfigMap, missing Prometheus CA certificate, RBAC issues
   - Verify ConfigMaps exist:
     ```bash
     kubectl get configmap -n workload-variant-autoscaler-system
     ```

3. **Resource constraints**: Check resource limits
   ```bash
   kubectl describe pod -n workload-variant-autoscaler-system \
     -l control-plane=controller-manager | grep -A 5 "Limits"
   ```

### Deployment Not Scaling

**Symptoms**: VariantAutoscaling CR shows desired replicas, but deployment doesn't scale.

**Diagnosis**:
```bash
# Check HPA status
kubectl get hpa -n <namespace>
kubectl describe hpa <hpa-name> -n <namespace>

# Check if WVA metrics are available
kubectl get --raw "/apis/external.metrics.k8s.io/v1beta1/namespaces/<namespace>/wva_desired_replicas"

# Check VariantAutoscaling status
kubectl describe variantautoscaling <name> -n <namespace>
```

**Solutions**:

1. **HPA not created**: Create an HPA resource that references WVA metrics
   ```yaml
   apiVersion: autoscaling/v2
   kind: HorizontalPodAutoscaler
   metadata:
     name: my-model-hpa
   spec:
     scaleTargetRef:
       apiVersion: apps/v1
       kind: Deployment
       name: my-model-deployment
     minReplicas: 1
     maxReplicas: 10
     metrics:
     - type: External
       external:
         metric:
           name: wva_desired_replicas
           selector:
             matchLabels:
               variant: my-model
         target:
           type: Value
           value: "1"
   ```

2. **Metrics not available**: Verify Prometheus Adapter is configured
   ```bash
   # Check if Prometheus Adapter is running
   kubectl get deployment -n monitoring prometheus-adapter
   
   # Check adapter logs
   kubectl logs -n monitoring deployment/prometheus-adapter
   ```

3. **HPA in cooldown**: Check stabilization window
   ```bash
   kubectl get hpa <name> -n <namespace> -o yaml | grep -A 5 behavior
   ```
   
   Recommended HPA configuration:
   ```yaml
   behavior:
     scaleDown:
       stabilizationWindowSeconds: 120
       policies:
       - type: Percent
         value: 50
         periodSeconds: 60
   ```

### Prometheus Not Scraping Metrics

**Symptoms**: WVA controller running but no metrics in Prometheus.

**Diagnosis**:
```bash
# Check ServiceMonitor
kubectl get servicemonitor -n <namespace>

# Check if Prometheus Operator is running
kubectl get pods -n monitoring -l app.kubernetes.io/name=prometheus

# Access Prometheus UI and check targets
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
# Navigate to http://localhost:9090/targets
```

**Solutions**:

1. **ServiceMonitor not created**: Ensure ServiceMonitor exists for vLLM pods
   ```bash
   kubectl apply -f - <<EOF
   apiVersion: monitoring.coreos.com/v1
   kind: ServiceMonitor
   metadata:
     name: vllm-metrics
     namespace: <namespace>
   spec:
     selector:
       matchLabels:
         app: vllm
     endpoints:
     - port: metrics
       interval: 30s
   EOF
   ```

2. **Service not exposing metrics**: Verify vLLM service has metrics port
   ```bash
   kubectl get svc -n <namespace> -o yaml | grep -A 10 "ports:"
   ```

3. **Prometheus RBAC**: Ensure Prometheus has permissions to scrape
   ```bash
   kubectl get clusterrole prometheus-k8s -o yaml
   ```

4. **TLS certificate issues**: Check Prometheus CA certificate
   ```bash
   kubectl get configmap -n workload-variant-autoscaler-system prometheus-ca-cert
   ```

### VariantAutoscaling Status Shows "DeploymentNotFound"

**Symptoms**: VA status condition shows deployment is missing.

**Diagnosis**:
```bash
# Check VariantAutoscaling status
kubectl get variantautoscaling <name> -n <namespace> -o yaml

# Verify deployment exists
kubectl get deployment <deployment-name> -n <namespace>

# Check scaleTargetRef in VA spec
kubectl get variantautoscaling <name> -n <namespace> -o jsonpath='{.spec.scaleTargetRef}'
```

**Solutions**:

1. **Deployment doesn't exist**: Create the deployment or fix the name
   ```bash
   kubectl get deployments -n <namespace>
   ```

2. **Incorrect scaleTargetRef**: Update the VariantAutoscaling CR
   ```bash
   kubectl edit variantautoscaling <name> -n <namespace>
   ```
   
   Verify `spec.scaleTargetRef` matches your deployment:
   ```yaml
   spec:
     scaleTargetRef:
       apiVersion: apps/v1
       kind: Deployment
       name: <correct-deployment-name>
   ```

3. **RBAC permissions**: Ensure WVA can access deployments
   ```bash
   kubectl get clusterrole workload-variant-autoscaler-role -o yaml
   ```

### High Reconciliation Latency

**Symptoms**: Slow response to traffic changes, long delays between scaling decisions.

**Diagnosis**:
```bash
# Check controller metrics
kubectl port-forward -n workload-variant-autoscaler-system \
  svc/workload-variant-autoscaler-controller-manager-metrics-service 8080:8443

# Query reconciliation duration
curl -k https://localhost:8080/metrics | grep reconcile_duration
```

**Solutions**:

1. **High Prometheus query latency**: Optimize Prometheus
   - Check Prometheus resource usage
   - Reduce metric retention or increase resources
   - Use recording rules for complex queries

2. **Too many VariantAutoscaling resources**: Consider controller isolation
   - Use [Multi-Controller Isolation](multi-controller-isolation.md)
   - Deploy separate controllers per namespace or team

3. **Slow collector cache**: Adjust cache settings
   ```yaml
   # In controller ConfigMap
   collector:
     cacheEnabled: true
     cacheTTL: 30s
   ```

### Incorrect Replica Calculations

**Symptoms**: WVA suggests incorrect number of replicas (too high or too low).

**Diagnosis**:
```bash
# Check current saturation metrics
kubectl port-forward -n <namespace> <vllm-pod> 8000:8000
curl http://localhost:8000/metrics | grep -E "(cache_usage|queue_depth)"

# Check WVA status for calculations
kubectl get variantautoscaling <name> -n <namespace> -o jsonpath='{.status.desiredOptimizedAlloc}'
```

**Solutions**:

1. **Incorrect saturation thresholds**: Adjust in ConfigMap
   ```yaml
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: workload-variant-autoscaler-config
     namespace: workload-variant-autoscaler-system
   data:
     saturation-scaling.yaml: |
       saturationThreshold: 0.85  # Adjust based on workload
       minReplicas: 1
       maxReplicas: 10
   ```

2. **Model profile misconfiguration**: Verify ModelProfile in VA spec
   ```yaml
   spec:
     modelProfile:
       accelerators:
       - acc: "L40S"
         accCount: 1
         maxBatchSize: 256  # Ensure this matches actual vLLM config
   ```

3. **Stale metrics**: Check metric timestamps
   ```bash
   # Verify Prometheus has recent data
   kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
   # Query: vllm_cache_usage (check timestamp)
   ```

## Advanced Diagnostics

### Enable Debug Logging

Increase controller log verbosity:

```bash
# Edit deployment
kubectl edit deployment -n workload-variant-autoscaler-system \
  workload-variant-autoscaler-controller-manager

# Add or modify args:
args:
- --zap-log-level=debug
```

Or with Helm:
```bash
helm upgrade workload-variant-autoscaler ./charts/workload-variant-autoscaler \
  --set controller.logLevel=debug \
  --reuse-values
```

### Inspect Controller Events

```bash
# Watch controller events
kubectl get events -n workload-variant-autoscaler-system \
  --sort-by='.lastTimestamp' -w

# Watch VariantAutoscaling events
kubectl get events -n <namespace> \
  --field-selector involvedObject.kind=VariantAutoscaling -w
```

### Verify Prometheus Queries

Test WVA's Prometheus queries manually:

```bash
# Port-forward to Prometheus
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090

# Example query for KV cache usage:
# Navigate to http://localhost:9090/graph
# Query: avg by (pod) (vllm_cache_usage{namespace="<namespace>",deployment="<name>"})
```

### Check Metric Collection

Verify WVA is collecting metrics correctly:

```bash
# Enable debug logging and grep for metric collection
kubectl logs -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager \
  --tail=500 | grep -i "metrics collected"
```

### Test HPA Scaling Manually

Force HPA to scale by setting desired replicas:

```bash
# Scale deployment directly
kubectl scale deployment <name> -n <namespace> --replicas=3

# Watch HPA reconciliation
kubectl get hpa <name> -n <namespace> -w
```

## Configuration Validation

### Validate VariantAutoscaling CR

```bash
# Dry-run to validate syntax
kubectl apply -f variantautoscaling.yaml --dry-run=client

# Validate against CRD schema
kubectl apply -f variantautoscaling.yaml --validate=true
```

### Validate Helm Configuration

```bash
# Lint Helm chart
helm lint ./charts/workload-variant-autoscaler

# Template and check output
helm template workload-variant-autoscaler ./charts/workload-variant-autoscaler \
  --values values.yaml \
  --debug
```

## Performance Tuning

### Optimize Reconciliation Frequency

Adjust controller sync period:

```yaml
# In controller ConfigMap
controller:
  syncPeriod: 60s  # Balance between responsiveness and load
```

### Tune Prometheus Scrape Intervals

```yaml
# In ServiceMonitor
spec:
  endpoints:
  - port: metrics
    interval: 30s  # Increase to reduce Prometheus load
```

### Adjust HPA Sync Period

```yaml
# In HPA spec
behavior:
  scaleDown:
    stabilizationWindowSeconds: 180
  scaleUp:
    stabilizationWindowSeconds: 0
    policies:
    - type: Pods
      value: 2
      periodSeconds: 60
```

## Getting More Help

If you've tried the solutions above and still have issues:

1. **Gather diagnostics**:
   ```bash
   # Save this output
   kubectl describe variantautoscaling <name> -n <namespace> > va-status.txt
   kubectl logs -n workload-variant-autoscaler-system \
     deployment/workload-variant-autoscaler-controller-manager \
     --tail=200 > controller-logs.txt
   kubectl get hpa <name> -n <namespace> -o yaml > hpa.yaml
   ```

2. **Check GitHub Issues**: Search for [similar issues](https://github.com/llm-d-incubation/workload-variant-autoscaler/issues)

3. **Open an Issue**: Include:
   - WVA version (from deployment image tag)
   - Kubernetes/OpenShift version
   - Diagnostic output from step 1
   - Steps to reproduce

4. **Join Community**: Attend [llm-d autoscaling community meetings](https://join.slack.com/share/enQtOTg1MzkwODExNDI5Mi02NWQwOWEwOWM4Y2Y3MTc4OTQyY2Y1ZDVlZmU2MjBmZDUwNjJhZGM3MjY4ZTQ5OTdjZjgzMmI0NjI0ZTBhZTM4)

## Related Documentation

- [FAQ](faq.md) - Common questions and answers
- [Installation Guide](installation.md) - Setup instructions
- [Configuration Guide](configuration.md) - Detailed configuration options
- [HPA Integration](../integrations/hpa-integration.md) - HPA setup and tuning
- [Prometheus Integration](../integrations/prometheus.md) - Metrics configuration
- [Developer Guide](../developer-guide/debugging.md) - Advanced debugging techniques
