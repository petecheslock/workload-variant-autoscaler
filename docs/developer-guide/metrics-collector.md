# Metrics Collector

## Overview

The metrics collector subsystem is responsible for gathering real-time metrics from inference servers and cluster resources. It provides a pluggable architecture that supports multiple backends while maintaining a consistent interface.

## Architecture

```
┌─────────────────────────────────────────────────┐
│          MetricsCollector Interface             │
│  (internal/interfaces/metrics_collector.go)    │
└────────────────┬────────────────────────────────┘
                 │
                 │ implements
                 │
    ┌────────────┴─────────────┐
    │                          │
┌───▼──────────────┐  ┌───────▼──────────────┐
│  Prometheus      │  │   EPP Collector      │
│  Collector       │  │   (Future)           │
└───┬──────────────┘  └──────────────────────┘
    │
    │ uses
    │
┌───▼──────────────────────────────────────┐
│  Cache Layer (internal/collector/cache)  │
│  - MemoryCache                            │
│  - TTL-based eviction                     │
│  - Thread-safe operations                 │
└───────────────────────────────────────────┘
```

## Collector Interface

```go
// MetricsCollector defines the interface for collecting metrics
type MetricsCollector interface {
    // FetchSaturationMetrics retrieves saturation metrics for all replicas
    FetchSaturationMetrics(ctx context.Context, modelID, namespace string) ([]ReplicaMetrics, error)
    
    // ValidateMetricsAvailability checks if metrics are available
    ValidateMetricsAvailability(ctx context.Context, modelID, namespace string) error
}
```

## Prometheus Collector

The Prometheus collector is the primary implementation, designed to work with vLLM inference servers that expose Prometheus metrics.

### Configuration

```go
type PrometheusConfig struct {
    URL              string        // Prometheus server URL
    CACertPath       string        // Path to CA certificate
    QueryTimeout     time.Duration // Query timeout (default: 30s)
    CacheTTL         time.Duration // Cache TTL (default: 10s)
    BackgroundFetch  bool          // Enable background fetching
    FetchInterval    time.Duration // Background fetch interval (default: 30s)
}
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PROMETHEUS_URL` | - | Prometheus server URL (required) |
| `PROMETHEUS_CA_CERT_PATH` | - | Path to CA certificate for TLS |
| `PROMETHEUS_QUERY_TIMEOUT` | `30s` | Query timeout duration |
| `METRICS_CACHE_TTL` | `10s` | Cache time-to-live |
| `METRICS_BACKGROUND_FETCH` | `true` | Enable background fetching |
| `METRICS_FETCH_INTERVAL` | `30s` | Background fetch interval |

### Metrics Collected

#### Saturation Metrics

**KV-Cache Utilization:**
```promql
max_over_time(vllm:gpu_cache_usage_perc{
    model_name="$modelID",
    namespace="$namespace"
}[1m])
```

**Queue Depth:**
```promql
max_over_time(vllm:num_requests_waiting{
    model_name="$modelID",
    namespace="$namespace"
}[1m])
```

**Request Rate:**
```promql
rate(vllm:request_success_total{
    model_name="$modelID",
    namespace="$namespace"
}[1m])
```

**Average Tokens:**
```promql
# Input tokens
avg_over_time(vllm:request_prompt_tokens{
    model_name="$modelID",
    namespace="$namespace"
}[1m])

# Output tokens
avg_over_time(vllm:request_generation_tokens{
    model_name="$modelID",
    namespace="$namespace"
}[1m])
```

#### Pod Metadata

The collector enriches metrics with Kubernetes pod metadata:
- **Variant Name:** Extracted from pod labels or deployment name
- **Accelerator Type:** Inferred from node labels or resource requests
- **Replica ID:** Pod name or index
- **Readiness:** Pod ready status

### Query Strategy

#### Peak Value Approach

The collector uses `max_over_time[1m]` for safety-critical metrics:
- **Why:** Captures worst-case scenarios to prevent capacity exhaustion
- **Metrics:** KV-cache utilization, queue depth
- **Trade-off:** More aggressive scaling vs. missing transient spikes

#### Average Value Approach

Uses `avg_over_time[1m]` for load characteristics:
- **Why:** Smooths out transients for stable scaling decisions
- **Metrics:** Request rate, token counts
- **Trade-off:** Better stability vs. slower response to changes

### Caching Layer

The collector implements multi-level caching to reduce Prometheus query load.

#### Cache Types

**L1: Query Result Cache**
- Caches entire query results
- TTL: 10 seconds (configurable)
- Key: `{modelID}:{namespace}:{queryType}`

**L2: Parsed Metrics Cache**
- Caches parsed ReplicaMetrics structures
- TTL: 5 seconds (shorter for freshness)
- Key: `{modelID}:{namespace}:parsed`

#### Cache Operations

```go
// Check cache before querying
cachedMetrics, ok := collector.cache.Get(cacheKey)
if ok && time.Since(cachedMetrics.Timestamp) < cacheTTL {
    return cachedMetrics.Data, nil
}

// Query Prometheus
result, err := collector.promAPI.Query(ctx, query, time.Now())

// Update cache
collector.cache.Set(cacheKey, CachedData{
    Data:      result,
    Timestamp: time.Now(),
})
```

#### Cache Invalidation

Caches are invalidated on:
1. **TTL Expiration:** Automatic after configured duration
2. **Manual Invalidation:** On error or staleness detection
3. **Capacity Limits:** LRU eviction when size exceeds limit

### Background Fetching

Background fetching pre-warms the cache to improve reconciliation latency.

#### Architecture

```
┌──────────────────┐
│  Background      │
│  Goroutine       │◄────┐
└────────┬─────────┘     │
         │                │ ticker
         │                │
         ▼                │
┌──────────────────┐     │
│  Fetch Metrics   │─────┘
│  for Active VAs  │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Update Cache    │
└──────────────────┘
```

#### Configuration

```go
// Enable background fetching
collector := prometheus.NewCollector(promAPI, k8sClient, prometheus.CollectorConfig{
    BackgroundFetch: true,
    FetchInterval:   30 * time.Second,
})

// Start background fetcher
go collector.StartBackgroundFetcher(ctx, variantAutoscalingList)
```

#### Benefits

- **Lower Reconciliation Latency:** Metrics pre-cached before controller needs them
- **Reduced Query Storms:** Spreads queries over time
- **Better Prometheus Load:** Predictable query pattern

#### Trade-offs

- **Resource Usage:** Additional goroutine and memory
- **Staleness Risk:** Cache may be slightly outdated
- **Complexity:** Additional failure modes

### Error Handling

#### Transient Errors

**Strategy:** Retry with exponential backoff

```go
retries := []time.Duration{1s, 2s, 4s}
for _, delay := range retries {
    result, err := query(ctx)
    if err == nil {
        return result, nil
    }
    time.Sleep(delay)
}
return nil, fmt.Errorf("max retries exceeded")
```

#### Permanent Errors

**Strategy:** Fail fast and record event

```go
if errors.Is(err, ErrPrometheusUnavailable) {
    recorder.Event(va, corev1.EventTypeWarning, 
        "MetricsUnavailable", 
        "Prometheus server unreachable")
    return nil, err
}
```

#### Missing Metrics

**Strategy:** Distinguish between no data and stale data

```go
if len(samples) == 0 {
    // No replicas reporting yet (scale-from-zero scenario)
    return []ReplicaMetrics{}, nil
}

if time.Since(latestTimestamp) > staleness Threshold {
    // Metrics exist but are stale
    return nil, ErrStaleMetrics
}
```

### Metric Validation

The collector validates metrics before returning them:

#### Completeness Check
```go
func validateMetrics(metrics []ReplicaMetrics) error {
    for _, m := range metrics {
        if m.KvCacheUsage < 0 || m.KvCacheUsage > 100 {
            return fmt.Errorf("invalid KV-cache: %f", m.KvCacheUsage)
        }
        if m.QueueDepth < 0 {
            return fmt.Errorf("invalid queue depth: %d", m.QueueDepth)
        }
    }
    return nil
}
```

#### Staleness Check
```go
func checkStaleness(timestamp time.Time) error {
    age := time.Since(timestamp)
    if age > 2*time.Minute {
        return fmt.Errorf("metrics stale: %v old", age)
    }
    return nil
}
```

## Usage Examples

### Basic Usage

```go
// Create Prometheus client
promClient, err := prometheus.NewClient(prometheus.ClientConfig{
    URL:        os.Getenv("PROMETHEUS_URL"),
    CACertPath: os.Getenv("PROMETHEUS_CA_CERT_PATH"),
})

// Create collector
collector := prometheus.NewCollector(
    promClient.API(),
    k8sClient,
    prometheus.CollectorConfig{
        QueryTimeout: 30 * time.Second,
        CacheTTL:     10 * time.Second,
    },
)

// Fetch metrics
metrics, err := collector.FetchSaturationMetrics(ctx, "meta/llama-3.1-8b", "default")
if err != nil {
    return fmt.Errorf("failed to fetch metrics: %w", err)
}

// Process metrics
for _, m := range metrics {
    log.Info("Replica metrics",
        "replicaID", m.ReplicaID,
        "kvCache", m.KvCacheUsage,
        "queueDepth", m.QueueDepth,
    )
}
```

### With Background Fetching

```go
// Create collector with background fetching
collector := prometheus.NewCollector(promAPI, k8sClient, prometheus.CollectorConfig{
    BackgroundFetch: true,
    FetchInterval:   30 * time.Second,
})

// Start background fetcher
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go collector.StartBackgroundFetcher(ctx, func() []types.NamespacedName {
    // Return list of active VariantAutoscaling resources
    return getActiveVariantAutoscalings()
})

// Metrics will be pre-cached in background
// Reconciliation will hit cache most of the time
metrics, err := collector.FetchSaturationMetrics(ctx, modelID, namespace)
```

### Factory Pattern

```go
import "github.com/llm-d-incubation/workload-variant-autoscaler/internal/collector/factory"

// Create collector using factory
collector, err := factory.NewMetricsCollector(
    factory.PrometheusBackend,
    promClient,
    k8sClient,
)
if err != nil {
    return fmt.Errorf("failed to create collector: %w", err)
}

// Future: Support for other backends
collector, err := factory.NewMetricsCollector(
    factory.EPPBackend,
    eppClient,
    k8sClient,
)
```

## Testing

### Unit Tests

```go
// Use noop cache for deterministic tests
cache := cache.NewNoopCache()
collector := prometheus.NewCollectorWithCache(promAPI, k8sClient, cache)

// Mock Prometheus responses
mockPromAPI.EXPECT().Query(ctx, gomock.Any(), gomock.Any()).
    Return(mockResult, nil)

metrics, err := collector.FetchSaturationMetrics(ctx, modelID, namespace)
assert.NoError(t, err)
assert.Len(t, metrics, expectedCount)
```

### Integration Tests

```go
// Use real Prometheus with test data
testProm := prometheustest.NewServer()
defer testProm.Close()

// Inject test metrics
testProm.InjectMetric("vllm:gpu_cache_usage_perc", 75.0, labels)
testProm.InjectMetric("vllm:num_requests_waiting", 3, labels)

// Create collector pointing to test Prometheus
collector := prometheus.NewCollector(testProm.API(), k8sClient, config)

// Verify metrics collected correctly
metrics, err := collector.FetchSaturationMetrics(ctx, modelID, namespace)
assert.NoError(t, err)
assert.Equal(t, 75.0, metrics[0].KvCacheUsage)
```

## Future Enhancements

### Planned Features

1. **EPP Backend:** Support for Elastic Performance Platform metrics
2. **Multi-Cluster:** Federated metric collection across clusters
3. **Custom Metrics:** User-defined metric queries
4. **Metric Aggregation:** Pre-aggregation for large-scale deployments
5. **Adaptive Caching:** Dynamic TTL based on metric variance

### Extensibility

To add a new metrics backend:

1. Implement `MetricsCollector` interface
2. Add factory method in `internal/collector/factory`
3. Add configuration validation
4. Write comprehensive tests
5. Document usage and configuration

## Troubleshooting

### Metrics Not Available

**Symptom:** `ValidateMetricsAvailability` returns error

**Possible Causes:**
- Prometheus server unreachable
- vLLM metrics not configured
- Incorrect model ID or namespace

**Resolution:**
```bash
# Check Prometheus connectivity
curl -k https://$PROMETHEUS_URL/api/v1/query?query=up

# Verify vLLM metrics exist
curl -k https://$PROMETHEUS_URL/api/v1/query?query=vllm:gpu_cache_usage_perc

# Check WVA logs
kubectl logs -n wva-system deployment/wva-controller | grep collector
```

### Stale Metrics

**Symptom:** Collector returns `ErrStaleMetrics`

**Possible Causes:**
- vLLM pods not running
- Prometheus scrape interval too long
- Network issues between Prometheus and vLLM

**Resolution:**
```bash
# Check vLLM pod status
kubectl get pods -n $NAMESPACE -l app=vllm

# Verify metric timestamps
kubectl port-forward -n $NAMESPACE svc/vllm-service 8080:8080
curl localhost:8080/metrics | grep vllm:gpu_cache_usage_perc

# Check Prometheus scrape config
kubectl get configmap -n monitoring prometheus-config -o yaml
```

### High Prometheus Load

**Symptom:** Prometheus queries slow or timing out

**Possible Causes:**
- Too frequent queries
- No caching enabled
- Large number of VariantAutoscaling resources

**Resolution:**
```yaml
# Enable caching and background fetch
env:
  - name: METRICS_CACHE_TTL
    value: "30s"
  - name: METRICS_BACKGROUND_FETCH
    value: "true"
  - name: METRICS_FETCH_INTERVAL
    value: "60s"
```

## Related Documentation

- [Architecture Overview](../design/architecture.md) - System design
- [Saturation Analyzer](../saturation-analyzer.md) - How metrics are analyzed
- [Prometheus Integration](../integrations/prometheus.md) - Prometheus setup
- [Package Reference](package-reference.md) - Internal package details

## References

- [Prometheus Query API](https://prometheus.io/docs/prometheus/latest/querying/api/)
- [vLLM Metrics](https://docs.vllm.ai/en/latest/observability/metrics.html)
- [Kubernetes Metrics](https://kubernetes.io/docs/concepts/cluster-administration/system-metrics/)
