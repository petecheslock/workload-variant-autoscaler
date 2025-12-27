# Troubleshooting Guide

This guide helps you diagnose and resolve common issues with the Workload-Variant-Autoscaler (WVA).

## Quick Diagnostics

### Health Check Checklist

Run through this checklist to quickly assess WVA health:

```bash
# 1. Check WVA controller status
kubectl get pods -n workload-variant-autoscaler-system

# 2. Check VariantAutoscaling resources
kubectl get variantautoscaling -A

# 3. Verify metrics availability
kubectl describe variantautoscaling <name> -n <namespace> | grep -A 5 Conditions

# 4. Check controller logs
kubectl logs -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager --tail=100

# 5. Verify Prometheus connectivity
kubectl get pods -n prometheus

# 6. Check HPA/KEDA status
kubectl get hpa -A
# OR
kubectl get scaledobject -A
```

## Common Issues

### 1. Controller Not Starting

#### Symptoms
- Controller pod in CrashLoopBackOff or Error state
- Pod restarts frequently

#### Diagnosis
```bash
# Check pod status
kubectl get pods -n workload-variant-autoscaler-system

# View pod logs
kubectl logs -n workload-variant-autoscaler-system \
  <pod-name> --previous

# Describe pod for events
kubectl describe pod -n workload-variant-autoscaler-system <pod-name>
```

#### Common Causes & Solutions

**Missing CRDs:**
```bash
# Apply CRDs manually
kubectl apply -f charts/workload-variant-autoscaler/crds/

# Verify CRDs installed
kubectl get crd variantautoscalings.llmd.ai
```

**RBAC permission issues:**
```bash
# Verify ServiceAccount exists
kubectl get serviceaccount -n workload-variant-autoscaler-system

# Check Role and RoleBinding
kubectl get clusterrole | grep variantautoscaling
kubectl get clusterrolebinding | grep variantautoscaling
```

**Resource limits:**
```yaml
# Increase controller resources in values.yaml
resources:
  limits:
    memory: 512Mi
    cpu: 1000m
  requests:
    memory: 256Mi
    cpu: 500m
```

### 2. Metrics Not Available

#### Symptoms
- `MetricsAvailable: False` in VariantAutoscaling status
- Controller logs show Prometheus connection errors

#### Diagnosis
```bash
# Check VariantAutoscaling status
kubectl describe variantautoscaling <name> -n <namespace>

# Check controller logs for Prometheus errors
kubectl logs -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager | \
  grep -i prometheus

# Verify Prometheus is running
kubectl get pods -n <prometheus-namespace>
```

#### Solutions

**Prometheus not running:**
```bash
# Check Prometheus deployment
kubectl get deployment -n prometheus

# Restart Prometheus if needed
kubectl rollout restart deployment prometheus-k8s -n prometheus
```

**Incorrect Prometheus URL:**
```bash
# Check controller configuration
kubectl get deployment -n workload-variant-autoscaler-system \
  workload-variant-autoscaler-controller-manager -o yaml | \
  grep PROMETHEUS_URL

# Update Prometheus URL in Helm values
helm upgrade workload-variant-autoscaler ./charts/workload-variant-autoscaler \
  --set prometheus.url="https://prometheus-k8s.prometheus:9091"
```

**TLS certificate issues:**
```bash
# Verify prometheus-ca-cert ConfigMap exists
kubectl get configmap prometheus-ca-cert -n workload-variant-autoscaler-system

# Check certificate validity
kubectl get configmap prometheus-ca-cert -n workload-variant-autoscaler-system \
  -o jsonpath='{.data.ca\.crt}' | openssl x509 -text -noout

# Recreate ConfigMap with correct certificate
kubectl create configmap prometheus-ca-cert \
  --from-file=ca.crt=/path/to/prometheus-ca.crt \
  -n workload-variant-autoscaler-system --dry-run=client -o yaml | \
  kubectl apply -f -
```

**Network connectivity:**
```bash
# Test connectivity from controller pod
kubectl exec -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager -- \
  curl -k https://prometheus-k8s.prometheus:9091/api/v1/query?query=up

# Check NetworkPolicies
kubectl get networkpolicy -n workload-variant-autoscaler-system
kubectl get networkpolicy -n <prometheus-namespace>
```

### 3. No vLLM Metrics Found

#### Symptoms
- Controller logs show "no metrics found for model"
- `kubectl get variantautoscaling` shows 0 current replicas

#### Diagnosis
```bash
# Check if vLLM pods are running
kubectl get pods -n <namespace> -l model_id=<your-model-id>

# Query Prometheus directly for vLLM metrics
kubectl port-forward -n prometheus svc/prometheus-k8s 9091:9091

# In another terminal, query metrics
curl -k "https://localhost:9091/api/v1/query?query=vllm:kv_cache_usage_perc"
```

#### Solutions

**vLLM pods not running:**
```bash
# Check deployment status
kubectl get deployment -n <namespace>

# Scale deployment if needed
kubectl scale deployment <deployment-name> -n <namespace> --replicas=1
```

**Missing metrics labels:**

vLLM metrics must include:
- `model_id` label matching VariantAutoscaling spec
- `pod` or `pod_name` label
- `namespace` label

Verify with:
```bash
# Check metric labels
curl -k "https://localhost:9091/api/v1/query?query=vllm:kv_cache_usage_perc" | jq
```

**Prometheus not scraping vLLM:**
```bash
# Check ServiceMonitor exists
kubectl get servicemonitor -n <namespace>

# Verify Prometheus configuration
kubectl get prometheus -n prometheus -o yaml | grep serviceMonitorSelector

# Check Prometheus targets
# Access Prometheus UI and navigate to Status > Targets
kubectl port-forward -n prometheus svc/prometheus-k8s 9091:9091
# Open http://localhost:9091/targets
```

### 4. Deployment Not Scaling

#### Symptoms
- WVA recommends scaling but deployment replicas don't change
- HPA/KEDA shows correct metric but doesn't scale

#### Diagnosis
```bash
# Check HPA status
kubectl get hpa <hpa-name> -n <namespace> -o yaml

# OR check KEDA ScaledObject
kubectl get scaledobject <scaledobject-name> -n <namespace> -o yaml

# Check WVA metrics
kubectl get --raw "/apis/custom.metrics.k8s.io/v1beta1/namespaces/<namespace>/pods/*/inferno_desired_replicas" | jq

# View HPA/KEDA events
kubectl describe hpa <hpa-name> -n <namespace>
kubectl describe scaledobject <scaledobject-name> -n <namespace>
```

#### Solutions

**Prometheus Adapter not running:**
```bash
# Check Prometheus Adapter deployment
kubectl get deployment -n prometheus-adapter

# Restart if needed
kubectl rollout restart deployment prometheus-adapter -n prometheus-adapter
```

**Metric not registered:**
```bash
# List available custom metrics
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1 | jq

# Check Prometheus Adapter configuration
kubectl get configmap prometheus-adapter -n prometheus-adapter -o yaml
```

**HPA target misconfigured:**

HPA metric target should typically be **1.0** for WVA:

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: my-model-hpa
spec:
  metrics:
  - type: Pods
    pods:
      metric:
        name: inferno_desired_replicas
      target:
        type: AverageValue
        averageValue: "1"  # Target = 1.0
```

**Deployment maxReplicas reached:**
```bash
# Check HPA maxReplicas
kubectl get hpa <hpa-name> -n <namespace> -o jsonpath='{.spec.maxReplicas}'

# Increase if needed
kubectl patch hpa <hpa-name> -n <namespace> --type='json' \
  -p='[{"op": "replace", "path": "/spec/maxReplicas", "value":10}]'
```

**scaleTargetRef mismatch:**
```bash
# Verify VariantAutoscaling scaleTargetRef
kubectl get variantautoscaling <name> -n <namespace> -o yaml | grep -A 3 scaleTargetRef

# Verify deployment exists
kubectl get deployment <deployment-name> -n <namespace>
```

### 5. Slow Scaling Response

#### Symptoms
- Long delay between saturation detection and scaling action
- Deployment takes minutes to scale

#### Diagnosis
```bash
# Check reconciliation interval
kubectl get deployment -n workload-variant-autoscaler-system \
  workload-variant-autoscaler-controller-manager -o yaml | \
  grep RECONCILIATION_INTERVAL

# Check HPA sync period
kubectl get hpa <hpa-name> -n <namespace> -o yaml | grep -A 10 behavior

# Check pod startup time
kubectl describe pod <pod-name> -n <namespace> | grep -A 20 Events
```

#### Solutions

**Reduce reconciliation interval:**
```yaml
# In Helm values.yaml
env:
  - name: RECONCILIATION_INTERVAL
    value: "30s"  # Default: 60s
```

**Optimize HPA stabilization:**
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: my-model-hpa
spec:
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 0  # Scale up immediately
      policies:
      - type: Percent
        value: 100
        periodSeconds: 15
    scaleDown:
      stabilizationWindowSeconds: 120  # Wait 2 minutes before scale-down
```

**Optimize pod startup:**
- Use smaller container images
- Pre-pull images on nodes
- Optimize readiness probes
- Use init containers for model loading

### 6. Incorrect Scaling Decisions

#### Symptoms
- WVA scales up unnecessarily
- WVA doesn't scale down when capacity available

#### Diagnosis
```bash
# Check saturation thresholds
kubectl get configmap capacity-scaling-config \
  -n workload-variant-autoscaler-system -o yaml

# View controller decision logs
kubectl logs -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager | \
  grep -i "scaling decision"

# Query actual saturation metrics
# Access Prometheus and query:
# vllm:kv_cache_usage_perc{model_id="your-model"}
# vllm:num_requests_waiting{model_id="your-model"}
```

#### Solutions

**Adjust thresholds:**

Edit `capacity-scaling-config` ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: capacity-scaling-config
  namespace: workload-variant-autoscaler-system
data:
  default: |
    # Saturation thresholds (0.0-1.0)
    kvCacheThreshold: 0.80        # Default: 0.80
    queueLengthThreshold: 5       # Default: 5
    
    # Scale-up triggers
    kvSpareTrigger: 0.1           # Default: 0.1 (10%)
    queueSpareTrigger: 3          # Default: 3 requests
```

**More aggressive scaling (scale up sooner):**
```yaml
kvCacheThreshold: 0.70        # Lower threshold = earlier saturation detection
kvSpareTrigger: 0.15          # Higher trigger = scale up with more headroom
```

**More conservative scaling (reduce flapping):**
```yaml
kvCacheThreshold: 0.85        # Higher threshold = tolerate more utilization
kvSpareTrigger: 0.05          # Lower trigger = scale up only when critical
```

### 7. Multiple Variants Issues

#### Symptoms
- Only one variant scales, others ignored
- Wrong variant scales (expensive instead of cheap)

#### Diagnosis
```bash
# Check all variants for a model
kubectl get deployment -n <namespace> -l model_id=<your-model-id>

# Check cost configuration
kubectl get configmap accelerator-unitcost \
  -n workload-variant-autoscaler-system -o yaml

# View controller multi-variant decision logs
kubectl logs -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager | \
  grep -i "variant"
```

#### Solutions

**Configure variant costs:**

Create or update `accelerator-unitcost` ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: accelerator-unitcost
  namespace: workload-variant-autoscaler-system
data:
  costs: |
    A100-SXM4-80GB: 30.0
    H100-SXM5-80GB: 50.0
    L40S: 15.0
    A10G: 10.0
```

**Verify variant labels:**

Each variant deployment must have:
- `model_id` label matching VariantAutoscaling
- Unique deployment name
- Matching HPA/KEDA configuration

**Check VariantAutoscaling status:**
```bash
kubectl get variantautoscaling <name> -n <namespace> -o yaml | \
  grep -A 20 variantStatus
```

## Performance Issues

### High Memory Usage

#### Diagnosis
```bash
# Check controller memory usage
kubectl top pod -n workload-variant-autoscaler-system

# Check for memory leaks in logs
kubectl logs -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager | \
  grep -i "out of memory\|OOM"
```

#### Solutions

**Reduce metrics cache:**
```yaml
# In Helm values.yaml
env:
  - name: METRICS_CACHE_TTL
    value: "15s"  # Default: 30s
```

**Increase controller resources:**
```yaml
resources:
  limits:
    memory: 1Gi
    cpu: 2000m
```

### High Reconciliation Latency

#### Diagnosis
```bash
# Check reconciliation duration in logs
kubectl logs -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager | \
  grep "reconciliation completed"
```

#### Solutions

**Optimize Prometheus queries:**
- Reduce query time range
- Use more specific selectors
- Enable Prometheus query caching

**Increase controller workers:**
```yaml
# In controller configuration
maxConcurrentReconciles: 5  # Default: 1
```

## Configuration Issues

### ConfigMap Not Applied

#### Symptoms
- Changes to ConfigMap don't take effect
- Controller uses default values

#### Solution

Restart controller after ConfigMap changes:
```bash
kubectl rollout restart deployment \
  workload-variant-autoscaler-controller-manager \
  -n workload-variant-autoscaler-system
```

### Invalid Configuration Syntax

#### Diagnosis
```bash
# Check controller logs for validation errors
kubectl logs -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager | \
  grep -i "invalid\|error.*config"
```

#### Solution

Validate YAML syntax:
```bash
# Validate ConfigMap syntax
kubectl apply --dry-run=client -f <configmap.yaml>

# Check for specific field errors in controller logs
```

## Integration Issues

### Prometheus Adapter Issues

#### Symptoms
- Custom metrics API not responding
- HPA shows "unknown" metric value

#### Diagnosis
```bash
# Check Prometheus Adapter pods
kubectl get pods -n prometheus-adapter

# Test custom metrics API
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1 | jq

# Check Prometheus Adapter logs
kubectl logs -n prometheus-adapter deployment/prometheus-adapter
```

#### Solutions

**Restart Prometheus Adapter:**
```bash
kubectl rollout restart deployment prometheus-adapter -n prometheus-adapter
```

**Verify configuration:**
```bash
kubectl get configmap prometheus-adapter -n prometheus-adapter -o yaml
```

### KEDA Issues

#### Symptoms
- ScaledObject shows "Unknown" trigger status
- KEDA not scaling deployment

#### Diagnosis
```bash
# Check KEDA operator
kubectl get pods -n keda

# Check ScaledObject status
kubectl describe scaledobject <name> -n <namespace>

# Check KEDA logs
kubectl logs -n keda deployment/keda-operator
```

#### Solutions

**Verify Prometheus scaler configuration:**
```yaml
triggers:
- type: prometheus
  metadata:
    serverAddress: https://prometheus-k8s.prometheus:9091
    query: inferno_desired_replicas{namespace="<namespace>",variant_name="<variant>"}
    threshold: '1'
```

**Check TLS configuration:**

If Prometheus uses TLS, configure KEDA with CA cert:
```yaml
authenticationRef:
  name: keda-prometheus-auth
```

## Debugging Tips

### Enable Debug Logging

```bash
# Edit controller deployment to enable debug logs
kubectl set env deployment/workload-variant-autoscaler-controller-manager \
  -n workload-variant-autoscaler-system \
  LOG_LEVEL=debug
```

### Collect Diagnostics

```bash
#!/bin/bash
# Save to collect-diagnostics.sh

NAMESPACE="workload-variant-autoscaler-system"
OUTPUT_DIR="wva-diagnostics-$(date +%Y%m%d-%H%M%S)"

mkdir -p "$OUTPUT_DIR"

# Controller logs
kubectl logs -n "$NAMESPACE" deployment/workload-variant-autoscaler-controller-manager \
  --tail=1000 > "$OUTPUT_DIR/controller.log"

# Controller describe
kubectl describe deployment -n "$NAMESPACE" \
  workload-variant-autoscaler-controller-manager > "$OUTPUT_DIR/controller-describe.txt"

# VariantAutoscaling resources
kubectl get variantautoscaling -A -o yaml > "$OUTPUT_DIR/variantautoscalings.yaml"

# ConfigMaps
kubectl get configmap -n "$NAMESPACE" -o yaml > "$OUTPUT_DIR/configmaps.yaml"

# HPA/KEDA
kubectl get hpa -A -o yaml > "$OUTPUT_DIR/hpas.yaml"
kubectl get scaledobject -A -o yaml > "$OUTPUT_DIR/scaledobjects.yaml"

# Events
kubectl get events -n "$NAMESPACE" --sort-by='.lastTimestamp' > "$OUTPUT_DIR/events.txt"

echo "Diagnostics collected in $OUTPUT_DIR/"
```

### Test Prometheus Connectivity

```bash
# Port-forward Prometheus
kubectl port-forward -n prometheus svc/prometheus-k8s 9091:9091 &

# Test query from local machine
curl -k "https://localhost:9091/api/v1/query?query=vllm:kv_cache_usage_perc"

# Test from controller pod
kubectl exec -n workload-variant-autoscaler-system \
  deployment/workload-variant-autoscaler-controller-manager -- \
  curl -k "https://prometheus-k8s.prometheus:9091/api/v1/query?query=up"
```

## Getting Help

If you're still experiencing issues:

1. **Collect diagnostics** using the script above
2. **Search existing issues:** https://github.com/llm-d-incubation/workload-variant-autoscaler/issues
3. **Open a new issue** with:
   - WVA version
   - Kubernetes version
   - Deployment method (Helm, Kustomize, etc.)
   - Diagnostic logs and configuration
   - Steps to reproduce
4. **Join community discussions:**
   - Slack: [llm-d autoscaling community](https://join.slack.com/share/...)

## Related Documentation

- [FAQ](./faq.md) - Frequently asked questions
- [Configuration Guide](./configuration.md) - Configuration details
- [Installation Guide](./installation.md) - Setup instructions
- [Architecture Overview](../design/architecture-overview.md) - System architecture
- [Developer Guide](../developer-guide/development.md) - Development setup
