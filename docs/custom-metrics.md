# Custom Metrics

This document describes the custom metrics emitted by the Workload-Variant-Autoscaler (WVA) and how to use them for autoscaling and monitoring.

## Overview

WVA emits custom metrics to Prometheus that can be consumed by Horizontal Pod Autoscaler (HPA) or KEDA for scaling decisions. These metrics represent WVA's analysis of model server saturation and optimal replica counts.

## Metrics Catalog

### Scaling Metrics

#### `wva_desired_replicas`

The optimal number of replicas computed by WVA's capacity model.

**Type**: Gauge  
**Labels**:
- `variant`: Name of the VariantAutoscaling resource
- `namespace`: Kubernetes namespace
- `model`: Model ID from the VariantAutoscaling spec

**Usage**: Primary metric for HPA/KEDA scaling decisions.

**Example**:
```promql
wva_desired_replicas{variant="llama-8b-autoscaler",namespace="inference",model="meta/llama-3.1-8b"}
```

**HPA Configuration**:
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: llama-8b-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: llama-8b-vllm
  minReplicas: 1
  maxReplicas: 10
  metrics:
  - type: External
    external:
      metric:
        name: wva_desired_replicas
        selector:
          matchLabels:
            variant: llama-8b-autoscaler
      target:
        type: Value
        value: "1"
```

#### `wva_saturation_level`

Current saturation level of the model server (0.0 to 1.0).

**Type**: Gauge  
**Labels**:
- `variant`: Name of the VariantAutoscaling resource
- `namespace`: Kubernetes namespace
- `pod`: Pod name (per-replica metric)

**Interpretation**:
- `0.0 - 0.6`: Under-utilized, potential for scale-down
- `0.6 - 0.85`: Optimal utilization range
- `0.85 - 1.0`: High saturation, scale-up recommended
- `> 1.0`: Overloaded, immediate scale-up needed

**Example Query**:
```promql
# Average saturation across all replicas
avg(wva_saturation_level{variant="llama-8b-autoscaler"})

# Maximum saturation (most loaded replica)
max(wva_saturation_level{variant="llama-8b-autoscaler"})
```

### Status Metrics

#### `wva_current_replicas`

Current number of replicas for the target deployment.

**Type**: Gauge  
**Labels**:
- `variant`: Name of the VariantAutoscaling resource
- `namespace`: Kubernetes namespace

**Usage**: Tracking current state for comparison with desired replicas.

**Example**:
```promql
wva_current_replicas{variant="llama-8b-autoscaler",namespace="inference"}
```

#### `wva_last_scaling_decision_timestamp`

Timestamp of the last scaling decision made by WVA.

**Type**: Gauge (Unix timestamp)  
**Labels**:
- `variant`: Name of the VariantAutoscaling resource
- `namespace`: Kubernetes namespace

**Usage**: Monitoring WVA activity and detecting stale decisions.

**Example**:
```promql
# Time since last scaling decision (seconds)
time() - wva_last_scaling_decision_timestamp{variant="llama-8b-autoscaler"}
```

### Controller Metrics

#### `wva_reconcile_duration_seconds`

Duration of reconciliation loop execution.

**Type**: Histogram  
**Labels**:
- `controller`: Controller name (always "variantautoscaling")
- `result`: Result of reconciliation ("success", "error", "requeue")

**Buckets**: 0.001, 0.01, 0.1, 0.5, 1.0, 5.0, 10.0, 30.0, 60.0 seconds

**Usage**: Performance monitoring and troubleshooting.

**Example**:
```promql
# P95 reconciliation latency
histogram_quantile(0.95, 
  rate(wva_reconcile_duration_seconds_bucket[5m])
)

# Average reconciliation time
rate(wva_reconcile_duration_seconds_sum[5m]) / 
rate(wva_reconcile_duration_seconds_count[5m])
```

#### `wva_reconcile_errors_total`

Total number of reconciliation errors.

**Type**: Counter  
**Labels**:
- `controller`: Controller name
- `error_type`: Type of error (e.g., "prometheus_query_failed", "deployment_not_found")

**Usage**: Tracking controller health and identifying issues.

**Example**:
```promql
# Error rate per minute
rate(wva_reconcile_errors_total[1m])

# Errors by type
sum by (error_type) (wva_reconcile_errors_total)
```

#### `wva_collector_metrics_collected_total`

Number of metrics successfully collected from Prometheus.

**Type**: Counter  
**Labels**:
- `variant`: Name of the VariantAutoscaling resource
- `metric_type`: Type of metric collected (e.g., "cache_usage", "queue_depth")

**Usage**: Monitoring data collection health.

**Example**:
```promql
# Collection rate
rate(wva_collector_metrics_collected_total[5m])
```

## Using Custom Metrics with HPA

### Basic HPA Configuration

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: my-model-hpa
  namespace: inference
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-model-deployment
  minReplicas: 1
  maxReplicas: 20
  metrics:
  - type: External
    external:
      metric:
        name: wva_desired_replicas
        selector:
          matchLabels:
            variant: my-model-autoscaler
      target:
        type: Value
        value: "1"  # Always try to match WVA's desired count
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 180
      policies:
      - type: Percent
        value: 50
        periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 0
      policies:
      - type: Pods
        value: 2
        periodSeconds: 60
```

### Prerequisites

1. **Prometheus Adapter**: Must be installed and configured

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm upgrade -i prometheus-adapter prometheus-community/prometheus-adapter \
  -f prometheus-adapter-values.yaml
```

2. **Prometheus Adapter Configuration**:

```yaml
# prometheus-adapter-values.yaml
rules:
- seriesQuery: 'wva_desired_replicas'
  resources:
    overrides:
      namespace:
        resource: "namespace"
  name:
    matches: "^(.*)$"
    as: "${1}"
  metricsQuery: 'wva_desired_replicas{variant="<<.LabelMatchers>>"}'
```

See [HPA Integration](../integrations/hpa-integration.md) for complete configuration.

## Using Custom Metrics with KEDA

KEDA provides more advanced scaling capabilities including scale-to-zero.

### Basic KEDA ScaledObject

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: my-model-scaledobject
  namespace: inference
spec:
  scaleTargetRef:
    name: my-model-deployment
  minReplicaCount: 0
  maxReplicaCount: 20
  pollingInterval: 30
  cooldownPeriod: 180
  triggers:
  - type: prometheus
    metadata:
      serverAddress: http://prometheus.monitoring:9090
      metricName: wva_desired_replicas
      query: |
        wva_desired_replicas{variant="my-model-autoscaler"}
      threshold: "1"
```

See [KEDA Integration](../integrations/keda-integration.md) for complete configuration.

## Querying Metrics

### Via Prometheus UI

```bash
# Port-forward to Prometheus
kubectl port-forward -n monitoring svc/prometheus 9090:9090

# Navigate to http://localhost:9090/graph
```

### Via kubectl

```bash
# Query external metrics API
kubectl get --raw "/apis/external.metrics.k8s.io/v1beta1/namespaces/inference/wva_desired_replicas" | jq
```

### Via PromQL

**Current vs. Desired Replicas**:
```promql
wva_desired_replicas{variant="llama-8b-autoscaler"} - 
wva_current_replicas{variant="llama-8b-autoscaler"}
```

**Scaling Lag**:
```promql
# Time between WVA decision and actual scaling
(time() - wva_last_scaling_decision_timestamp) and 
(wva_desired_replicas != wva_current_replicas)
```

**Saturation Trend**:
```promql
# Rate of saturation change
rate(wva_saturation_level[5m])
```

## Monitoring & Alerting

### Recommended Alerts

#### High Saturation Alert

```yaml
alert: WVAHighSaturation
expr: |
  max(wva_saturation_level{}) by (variant, namespace) > 0.95
for: 5m
annotations:
  summary: "WVA variant {{ $labels.variant }} is highly saturated"
  description: "Saturation level is {{ $value }}, scaling may be needed"
```

#### Scaling Lag Alert

```yaml
alert: WVAScalingLag
expr: |
  abs(wva_desired_replicas - wva_current_replicas) > 2 and
  (time() - wva_last_scaling_decision_timestamp) > 300
for: 5m
annotations:
  summary: "WVA scaling is lagging for {{ $labels.variant }}"
  description: "Desired: {{ $labels.desired }}, Current: {{ $labels.current }}"
```

#### Stale Metrics Alert

```yaml
alert: WVAStaleMetrics
expr: |
  (time() - wva_last_scaling_decision_timestamp) > 600
for: 10m
annotations:
  summary: "WVA metrics are stale for {{ $labels.variant }}"
  description: "No scaling decision in last {{ $value }} seconds"
```

### Grafana Dashboards

See [Metrics & Health Monitoring](metrics-health-monitoring.md) for pre-built Grafana dashboard configurations.

## Metric Retention

Metrics are stored in Prometheus according to your retention policy:

```yaml
# In Prometheus configuration
storage:
  tsdb:
    retention.time: 15d
    retention.size: 50GB
```

For long-term storage, consider:
- **Thanos**: Long-term Prometheus storage
- **Cortex**: Multi-tenant Prometheus as a service
- **Victoria Metrics**: High-performance TSDB

## Troubleshooting

### Metrics Not Appearing

1. **Check WVA controller logs**:
   ```bash
   kubectl logs -n workload-variant-autoscaler-system \
     deployment/workload-variant-autoscaler-controller-manager | grep metrics
   ```

2. **Verify Prometheus is scraping WVA**:
   ```bash
   kubectl port-forward -n monitoring svc/prometheus 9090:9090
   # Check http://localhost:9090/targets
   ```

3. **Check ServiceMonitor**:
   ```bash
   kubectl get servicemonitor -n workload-variant-autoscaler-system
   ```

### HPA Not Reading Metrics

1. **Verify Prometheus Adapter**:
   ```bash
   kubectl get apiservice v1beta1.external.metrics.k8s.io
   kubectl get --raw /apis/external.metrics.k8s.io/v1beta1
   ```

2. **Test metric availability**:
   ```bash
   kubectl get --raw "/apis/external.metrics.k8s.io/v1beta1/namespaces/inference/wva_desired_replicas" | jq
   ```

3. **Check HPA status**:
   ```bash
   kubectl describe hpa my-model-hpa -n inference
   ```

## Related Documentation

- [HPA Integration](../integrations/hpa-integration.md) - Complete HPA setup
- [KEDA Integration](../integrations/keda-integration.md) - KEDA configuration
- [Prometheus Integration](../integrations/prometheus.md) - Prometheus setup
- [Metrics & Health Monitoring](metrics-health-monitoring.md) - Monitoring guide

---

For questions about custom metrics, open a GitHub issue or discussion.
