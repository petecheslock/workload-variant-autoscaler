# Metrics Collector

## Overview

The Metrics Collector is responsible for gathering performance and saturation metrics from inference servers via Prometheus. It provides efficient, cached metric collection with background fetching capabilities to minimize API load and latency.

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                   Collector Factory                      │
│  Creates appropriate collector based on configuration    │
└────────────────────────┬─────────────────────────────────┘
                         │
        ┌────────────────┴────────────────┐
        │                                  │
┌───────▼──────────┐          ┌───────────▼──────────┐
│   Prometheus     │          │    NoOp Collector    │
│   Collector      │          │   (Testing/Offline)  │
└───────┬──────────┘          └──────────────────────┘
        │
        ├── Saturation Metrics Collector
        ├── Model Metrics Collector (Future)
        └── Custom Metrics Collector (Future)
```

## Package Structure

```
internal/collector/
├── collector.go              # MetricsCollector interface
├── factory.go                # Factory for creating collectors
├── cache/                    # Caching layer
│   ├── cache.go             # Cache interface
│   ├── memory_cache.go      # In-memory implementation
│   └── noop_cache.go        # No-op for testing
├── config/                   # Collector configuration
│   └── config.go            # Config structures and validation
└── prometheus/              # Prometheus implementation
    ├── prometheus_collector.go       # Main collector
    ├── saturation_metrics.go        # vLLM saturation metrics
    ├── background_fetching.go       # Proactive cache warming
    ├── cache_operations.go          # Cache management
    ├── tracking.go                  # Deployment/pod tracking
    └── query_helpers.go             # Prometheus query utilities
```

## Components

### 1. MetricsCollector Interface

**Location**: `internal/collector/collector.go`

**Purpose**: Defines the contract for collecting metrics from various sources.

```go
type MetricsCollector interface {
    // CollectSaturationMetrics collects KV-cache and queue metrics
    CollectSaturationMetrics(ctx context.Context, va *VariantAutoscaling) 
        ([]ReplicaMetrics, error)
    
    // CollectModelMetrics collects performance metrics (future)
    CollectModelMetrics(ctx context.Context, va *VariantAutoscaling) 
        (*ModelMetrics, error)
    
    // GetPodMetrics returns metrics for a specific pod
    GetPodMetrics(ctx context.Context, namespace, podName string) 
        (*PodMetrics, error)
}
```

### 2. Prometheus Collector

**Location**: `internal/collector/prometheus/prometheus_collector.go`

**Purpose**: Implements MetricsCollector using Prometheus as the data source.

**Key Features**:
- ✅ TLS support with CA certificate validation
- ✅ Bearer token authentication
- ✅ Connection pooling and timeout configuration
- ✅ Configurable query timeout (default: 30s)
- ✅ PromQL injection protection

**Configuration**:
```go
type Config struct {
    PrometheusURL     string        // Prometheus endpoint
    PrometheusCAPath  string        // CA certificate path
    TokenPath         string        // Bearer token path
    InsecureSkipVerify bool         // Skip TLS verification (not recommended)
    QueryTimeout      time.Duration // Query timeout
}
```

**Example Usage**:
```go
promAPI, err := utils.NewPrometheusAPI(ctx, config)
if err != nil {
    return nil, err
}
collector := prometheus.NewPrometheusCollector(promAPI, client)
```

### 3. Saturation Metrics Collector

**Location**: `internal/collector/prometheus/saturation_metrics.go`

**Purpose**: Collects vLLM-specific saturation metrics (KV-cache utilization, queue depth).

**Metrics Collected**:

| Metric | PromQL Query | Purpose |
|--------|-------------|---------|
| `vllm:kv_cache_usage_perc` | `max_over_time[1m]` | KV-cache saturation |
| `vllm:num_requests_waiting` | `max_over_time[1m]` | Request queue depth |

**Why `max_over_time[1m]`?**
- Safety-first approach: captures peak utilization over 1 minute
- Prevents missed saturation spikes
- Smooths out transient fluctuations

**Data Enrichment**:
- Pod-to-deployment mapping via owner references
- Variant name extraction from labels
- Accelerator type detection
- Cost attribution per replica

**Example Query**:
```promql
max_over_time(
  vllm:kv_cache_usage_perc{
    namespace="llm-inference",
    pod=~"llama-8b-.*"
  }[1m]
)
```

**Output Structure**:
```go
type ReplicaMetrics struct {
    PodName         string
    VariantName     string
    AcceleratorName string
    Cost            float64
    KvCacheUsage    float64  // 0.0-1.0
    QueueLength     int      // Number of waiting requests
    Timestamp       time.Time
}
```

### 4. Caching Layer

**Location**: `internal/collector/cache/`

**Purpose**: Reduce Prometheus API load and improve latency through intelligent caching.

**Cache Interface**:
```go
type Cache interface {
    Get(key string) (interface{}, bool)
    Set(key string, value interface{}, ttl time.Duration)
    Delete(key string)
    Clear()
    Size() int
}
```

**Memory Cache Implementation**:
- Thread-safe with RWMutex
- TTL-based expiration
- LRU eviction (future enhancement)
- Configurable max size

**Configuration**:
```go
type CacheConfig struct {
    DefaultTTL time.Duration // Default: 30s
    MaxSize    int          // Default: unlimited
    Enabled    bool         // Default: true
}
```

**Cache Key Format**:
```
saturation:<namespace>/<va-name>
model:<namespace>/<va-name>
pod:<namespace>/<pod-name>
```

**Cache Behavior**:
- **Hit**: Return cached value immediately (~1ms)
- **Miss**: Query Prometheus, cache result (~100-500ms)
- **Expired**: Refresh from Prometheus, update cache
- **Error**: Return last known good value if available

### 5. Background Fetching

**Location**: `internal/collector/prometheus/background_fetching.go`

**Purpose**: Proactively warm cache before optimization cycles to eliminate cold-start latency.

**How It Works**:
1. Tracks active VariantAutoscaling resources
2. Spawns background goroutines per resource
3. Fetches metrics at configurable interval (default: 25s)
4. Updates cache proactively
5. Optimization cycle reads from warm cache (~1ms)

**Benefits**:
- Zero latency during optimization
- Reduced Prometheus load spikes
- Smooth, predictable performance

**Configuration**:
```go
type BackgroundFetchConfig struct {
    Enabled          bool          // Default: true
    FetchInterval    time.Duration // Default: 25s
    MaxConcurrent    int          // Default: 10
    ErrorBackoff     time.Duration // Default: 5s
}
```

**Example**:
```
Timeline:
T+0s:  Optimization cycle starts
T+0s:  Read from cache (1ms) - warm cache hit
T+25s: Background fetch updates cache
T+30s: Next optimization cycle
T+30s: Read from cache (1ms) - warm cache hit
```

### 6. Tracking and Discovery

**Location**: `internal/collector/prometheus/tracking.go`

**Purpose**: Discover and track deployments and pods for VariantAutoscaling resources.

**Functions**:
- **FindDeployment**: Locate target deployment by name or modelID inference
- **ListPods**: Get all pods for a deployment
- **GetOwnerReferences**: Resolve pod-to-deployment ownership chain
- **ExtractVariantName**: Parse variant name from pod labels/annotations

**Deployment Discovery Logic**:
1. Check `spec.scaleTargetRef` if specified
2. Infer from `spec.modelID` (e.g., `llama-8b` → `llama-8b-deployment`)
3. Search by labels: `llmd.ai/model-id=<modelID>`
4. Return error if ambiguous or not found

### 7. Query Helpers

**Location**: `internal/collector/prometheus/query_helpers.go`

**Purpose**: Utilities for constructing safe, efficient Prometheus queries.

**Functions**:
- **escapePrometheusLabelValue**: Prevent PromQL injection
- **buildMatchers**: Construct label matchers safely
- **parseMetricValue**: Extract numeric values from Prometheus responses
- **aggregateResults**: Combine results from multiple queries

**Example**:
```go
// Safe query construction
query := fmt.Sprintf(
    `max_over_time(vllm:kv_cache_usage_perc{namespace="%s",pod=~"%s"}[1m])`,
    escapePrometheusLabelValue(namespace),
    escapePrometheusLabelValue(podPattern),
)
```

## Data Flow

### Cold Start (First Collection)

```
1. CollectSaturationMetrics called
   ↓
2. Check cache → MISS
   ↓
3. Find target deployment
   ↓
4. List pods for deployment
   ↓
5. Query Prometheus for each pod
   - vllm:kv_cache_usage_perc
   - vllm:num_requests_waiting
   ↓
6. Enrich with metadata (variant, accelerator, cost)
   ↓
7. Store in cache (TTL: 30s)
   ↓
8. Return []ReplicaMetrics
```

**Latency**: ~500-1000ms (Prometheus query time + processing)

### Warm Cache (Subsequent Collections)

```
1. Background fetcher runs (T-5s)
   ↓
2. Fetches metrics proactively
   ↓
3. Updates cache
   ↓
4. CollectSaturationMetrics called (T+0s)
   ↓
5. Check cache → HIT
   ↓
6. Return cached []ReplicaMetrics immediately
```

**Latency**: ~1-5ms (cache lookup only)

## Configuration

### Collector Configuration

**ConfigMap**: `wva-config`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: wva-config
  namespace: workload-variant-autoscaler-system
data:
  config.yaml: |
    collector:
      type: prometheus
      prometheus:
        url: https://prometheus:9090
        caPath: /etc/prometheus/ca.crt
        tokenPath: /var/run/secrets/kubernetes.io/serviceaccount/token
        insecureSkipVerify: false
        queryTimeout: 30s
      cache:
        enabled: true
        defaultTTL: 30s
        maxSize: 1000
      backgroundFetch:
        enabled: true
        fetchInterval: 25s
        maxConcurrent: 10
```

### Prometheus Connection

**TLS Configuration**:
```go
// Create TLS config from CA certificate
tlsConfig, err := utils.TLSConfigFromCAFile(caPath)
if err != nil {
    return nil, err
}

// Create HTTP transport with TLS
transport := &http.Transport{
    TLSClientConfig: tlsConfig,
}

// Add bearer token
transport = utils.AddBearerToken(transport, tokenPath)
```

**Connection Pooling**:
- Max idle connections: 100
- Max connections per host: 10
- Idle timeout: 90s
- Connection timeout: 30s

## Error Handling

### Prometheus Query Errors

**Transient Errors** (retry):
- Network timeouts
- Connection refused
- DNS resolution failures

**Permanent Errors** (fail fast):
- Invalid query syntax
- Unknown metric names
- Authentication failures

**Retry Strategy**:
```go
maxRetries := 3
backoff := 100 * time.Millisecond
for i := 0; i < maxRetries; i++ {
    result, err := promAPI.Query(ctx, query, time.Now())
    if err == nil {
        return result, nil
    }
    time.Sleep(backoff)
    backoff *= 2
}
```

### Cache Failures

**Cache Miss**:
- Query Prometheus directly
- Update cache with result
- Log cache miss for monitoring

**Cache Eviction**:
- LRU eviction when max size reached
- TTL-based expiration
- Manual invalidation on VA updates

### Missing Metrics

**Scenarios**:
1. vLLM not exposing metrics → Log warning, return empty metrics
2. Pod not ready → Exclude from results
3. Deployment not found → Return error
4. No pods running → Return empty slice (not error)

## Performance Characteristics

### Latency

**Cold Start**:
- Prometheus query: 100-500ms
- Processing: 10-50ms
- Total: 110-550ms per VA

**Warm Cache**:
- Cache lookup: 1-5ms
- Processing: <1ms
- Total: 2-6ms per VA

### Throughput

**Without Caching**:
- ~2-10 queries/second per VA
- Limited by Prometheus API capacity

**With Caching**:
- ~200-1000 queries/second per VA
- Limited by memory bandwidth

### Resource Usage

**Memory**:
- Base: ~10MB
- Per cached VA: ~10KB
- 100 VAs with cache: ~11MB

**CPU**:
- Collection: <1% per VA
- Background fetching: <0.1% per VA
- Cache operations: negligible

### Prometheus Load

**Without Background Fetching**:
- 2 queries per VA per reconciliation (30s interval)
- 100 VAs = 200 queries/30s = ~7 QPS

**With Background Fetching**:
- 2 queries per VA per fetch interval (25s)
- 100 VAs = 200 queries/25s = ~8 QPS
- More predictable load distribution

## Monitoring and Observability

### Metrics Exposed

**Collector Metrics**:
```
wva_collector_cache_hits_total{type="saturation"}
wva_collector_cache_misses_total{type="saturation"}
wva_collector_prometheus_queries_total{status="success|error"}
wva_collector_query_duration_seconds{query_type="saturation"}
wva_collector_background_fetch_errors_total{va="namespace/name"}
```

### Logs

**Structured Logging**:
```go
log.Info("Collecting saturation metrics",
    "va", va.Name,
    "namespace", va.Namespace,
    "deployment", deployment.Name,
    "pods", len(pods),
)

log.Debug("Prometheus query",
    "query", query,
    "duration", duration,
    "results", len(results),
)

log.Error(err, "Failed to query Prometheus",
    "va", va.Name,
    "retries", retries,
)
```

**Log Levels**:
- `DEBUG`: Cache hits/misses, query details
- `INFO`: Collection start/end, deployment discovery
- `WARN`: Missing metrics, pod issues
- `ERROR`: Prometheus failures, invalid configurations

## Testing

### Unit Tests

**Coverage**:
- `collector_test.go`: Interface contract tests
- `prometheus_collector_test.go`: Prometheus client tests
- `saturation_metrics_test.go`: Metric parsing and enrichment
- `cache_operations_test.go`: Cache behavior
- `background_fetching_test.go`: Proactive fetching logic

**Mocking**:
```go
// Mock Prometheus API
type mockPrometheusAPI struct {
    queryFunc func(ctx, query, ts) (model.Value, error)
}

// Mock responses
func (m *mockPrometheusAPI) Query(ctx context.Context, query string, ts time.Time) (model.Value, error) {
    return m.queryFunc(ctx, query, ts)
}
```

### Integration Tests

**E2E Tests**: `test/e2e-saturation-based/`
- Real Prometheus instance
- vLLM emulator with metrics
- End-to-end metric collection and caching

## Troubleshooting

### Common Issues

**No metrics collected**:
- Check Prometheus connectivity: `curl -k https://prometheus:9090/-/healthy`
- Verify vLLM metrics exist: `curl http://vllm-pod:8000/metrics`
- Check RBAC permissions for Prometheus access

**Stale metrics**:
- Verify cache TTL: Should be < optimization interval
- Check background fetching enabled
- Ensure vLLM is actively serving requests

**High Prometheus load**:
- Enable caching: `collector.cache.enabled=true`
- Enable background fetching: `collector.backgroundFetch.enabled=true`
- Increase fetch interval: `collector.backgroundFetch.fetchInterval=60s`

**Authentication failures**:
- Verify CA certificate path: `ls -la /etc/prometheus/ca.crt`
- Check token file exists: `ls -la /var/run/secrets/kubernetes.io/serviceaccount/token`
- Validate token has Prometheus access

## Best Practices

### Configuration

1. **Always enable caching** for production environments
2. **Use background fetching** to eliminate cold-start latency
3. **Set cache TTL < optimization interval** (e.g., 30s TTL, 35s interval)
4. **Configure appropriate query timeout** based on cluster size
5. **Use TLS and authentication** for Prometheus in production

### Monitoring

1. **Monitor cache hit ratio**: Should be >90% with background fetching
2. **Track Prometheus query latency**: Alert if >1s consistently
3. **Watch for metric gaps**: Alert if no metrics for >2 minutes
4. **Monitor background fetch errors**: Should be near zero

### Scaling

1. **Limit concurrent background fetches**: Prevents Prometheus overload
2. **Adjust cache size** based on number of VAs (10KB per VA)
3. **Use multiple Prometheus replicas** for large clusters (100+ VAs)
4. **Consider thanos/cortex** for long-term metric storage

## Future Enhancements

### Planned Features

- [ ] LRU cache eviction
- [ ] Distributed caching (Redis integration)
- [ ] Adaptive cache TTL based on metric volatility
- [ ] Predictive prefetching using historical patterns
- [ ] Multi-Prometheus support with failover
- [ ] Metric aggregation across time windows
- [ ] Custom metric collectors (gRPC, REST APIs)

### Performance Improvements

- [ ] Batch queries to Prometheus
- [ ] Parallel pod metric collection
- [ ] Delta encoding for cached metrics
- [ ] Compression for large cache entries

## References

- [Prometheus API Documentation](https://prometheus.io/docs/prometheus/latest/querying/api/)
- [vLLM Metrics](https://docs.vllm.ai/en/latest/serving/metrics.html)
- [Saturation Engine Architecture](./architecture-engines.md)
- [Saturation Scaling Configuration](../saturation-scaling-config.md)

## Contributing

Improvements to collector efficiency welcome! See [Developer Guide](../developer-guide/development.md).

**Adding a new collector**:
1. Implement `MetricsCollector` interface
2. Add factory method in `factory.go`
3. Create comprehensive unit tests
4. Document configuration options
5. Add integration tests
