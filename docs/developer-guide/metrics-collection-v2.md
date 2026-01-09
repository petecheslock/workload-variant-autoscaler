# Metrics Collection Architecture (V2)

> **Status**: Active (as of v0.4.0, PR #558)
> 
> **Migration Note**: The legacy v1 `MetricsCollector` interface has been removed in favor of the v2 collector architecture.

## Overview

The WVA v2 metrics collection architecture provides a flexible, cache-aware, and extensible framework for collecting metrics from various backends. The primary implementation uses Prometheus, but the architecture supports pluggable metrics sources.

## Key Components

### 1. MetricsSource Interface (`internal/collector/v2/source.go`)

The core abstraction for metrics collection. All metrics sources implement this interface:

```go
type MetricsSource interface {
    // QueryList returns the list of available queries
    QueryList() *QueryList
    
    // Refresh fetches metrics based on the provided specification
    Refresh(ctx context.Context, spec RefreshSpec) (map[string]*MetricResult, error)
    
    // Get retrieves cached metric values
    Get(queryName string, params map[string]string) *CachedValue
}
```

### 2. PrometheusSource (`internal/collector/v2/prometheus_source.go`)

Production implementation that:
- Connects to Prometheus API
- Executes parameterized queries using templates
- Implements result caching with configurable TTL
- Supports metric staleness filtering
- Handles query failures gracefully

**Configuration Options**:
```go
type PrometheusSourceConfig struct {
    CacheTTL           time.Duration  // Default: 30s
    StalenessThreshold time.Duration  // Default: 2m
    MaxCacheSize       int            // Default: 1000
}
```

### 3. Query Templates (`internal/collector/v2/query_template.go`)

Parameterized Prometheus queries with variable substitution:

```go
// Example: KV cache usage query
kvCacheUsageQuery := &QueryTemplate{
    Name: "vllm_kv_cache_usage",
    Template: `max_over_time(vllm:cache_usage_percent{
        namespace="{{.namespace}}",
        pod=~"{{.deployment_name}}-.*"
    }[1m])`,
}
```

### 4. Source Registry (`internal/collector/v2/registry.go`)

Central registry for managing multiple metrics sources:

```go
registry := collectorv2.NewSourceRegistry()
promSource := collectorv2.NewPrometheusSource(ctx, promAPI, config)
registry.Register("prometheus", promSource)
```

### 5. Domain-Specific Collectors

High-level collectors that use `MetricsSource` for specific domains:

#### ReplicaMetricsCollector (`internal/engines/saturation/metrics/replica_metrics.go`)

Collects saturation metrics (KV cache, queue depth) for replica-level analysis:

```go
collector := NewReplicaMetricsCollector(promSource, k8sClient)
metrics, err := collector.CollectReplicaMetrics(
    ctx, 
    modelID, 
    namespace, 
    deployments, 
    variantAutoscalings, 
    variantCosts,
)
```

**Features**:
- Automatic pod-to-VariantAutoscaling mapping
- Metric staleness filtering (default: 2 minutes)
- Per-replica cost tracking
- Ready replica filtering

## Architecture Benefits

### 1. Separation of Concerns
- **Source Layer**: Raw metric fetching and caching
- **Collector Layer**: Domain-specific metric aggregation
- **Analysis Layer**: Business logic using collected metrics

### 2. Testability
- Mock implementations (`NoOpSource`) for testing
- Dependency injection via `MetricsSource` interface
- Isolated unit testing of each layer

### 3. Extensibility
- Easy to add new metrics sources (e.g., OpenMetrics, custom backends)
- Query templates simplify Prometheus query management
- Registry pattern supports multiple concurrent sources

### 4. Performance
- Built-in caching reduces Prometheus load
- Staleness filtering avoids stale data
- Configurable TTL balances freshness vs. performance

## Migration from V1

### What Changed (PR #558)

**Removed**:
- `internal/collector/collector.go` - Old `MetricsCollector` interface
- `internal/collector/prometheus/prometheus_collector.go` - Legacy implementation
- `COLLECTOR_V2` environment variable - No longer needed
- Manual cache invalidation on scaling events

**Added**:
- V2 collector is now the default and only metrics collection system
- Automatic metric staleness handling
- Improved caching strategy

### Code Changes Required

**Before (v1)**:
```go
metricsCollector, err := collector.NewMetricsCollector(collector.Config{
    Type:        collector.CollectorTypePrometheus,
    PromAPI:     promAPI,
    CacheConfig: cacheConfig,
})

// In controller
reconciler := &VariantAutoscalingReconciler{
    Client:           mgr.GetClient(),
    MetricsCollector: metricsCollector,
}
```

**After (v2)**:
```go
sourceRegistry := collectorv2.NewSourceRegistry()
promSource := collectorv2.NewPrometheusSource(ctx, promAPI, 
    collectorv2.DefaultPrometheusSourceConfig())
sourceRegistry.Register("prometheus", promSource)

// In engine
engine := saturation.NewEngine(
    mgr.GetClient(),
    mgr.GetScheme(),
    recorder,
    sourceRegistry,
)
```

## Query Registration

Saturation-specific queries are registered at startup:

```go
// internal/engines/saturation/metrics/register.go
func RegisterSaturationQueries(registry *collector.SourceRegistry) {
    promSource := registry.Get("prometheus")
    
    // Register KV cache query
    promSource.QueryList().Register(&collector.QueryTemplate{
        Name:     "vllm_kv_cache_usage",
        Template: `max_over_time(vllm:cache_usage_percent[1m])`,
    })
    
    // Register queue depth query
    promSource.QueryList().Register(&collector.QueryTemplate{
        Name:     "vllm_queue_depth",
        Template: `vllm:num_requests_waiting`,
    })
}
```

## Caching Strategy

### Cache Keys
Cache keys are constructed from query name and parameters:
```
queryName:param1=value1:param2=value2
```

### Cache Invalidation
- **Time-based**: Entries expire after `CacheTTL` (default: 30s)
- **Staleness filtering**: Metrics older than threshold are filtered
- **No manual invalidation**: Removed in PR #558 as automatic staleness handling is sufficient

### Cache Behavior
1. On `Refresh()`: Fetch fresh metrics, update cache
2. On `Get()`: Return cached value if within TTL, otherwise nil
3. Stale metrics are filtered during `CollectReplicaMetrics()`

## Testing

### Unit Testing with NoOpSource

```go
func TestWithoutMetrics(t *testing.T) {
    sourceRegistry := collectorv2.NewSourceRegistry()
    sourceRegistry.Register("prometheus", collectorv2.NewNoOpSource())
    
    engine := saturation.NewEngine(client, scheme, recorder, sourceRegistry)
    // Test engine behavior without actual metrics
}
```

### Integration Testing with Mock Prometheus

```go
mockPromAPI := &MockPromAPI{
    QueryResults: map[string]model.Value{
        "vllm:cache_usage_percent": model.Vector{...},
    },
}

promSource := collectorv2.NewPrometheusSource(ctx, mockPromAPI, config)
registry.Register("prometheus", promSource)
```

## Configuration

### Environment Variables

None required for v2 collector (previously `COLLECTOR_V2` was used).

### ConfigMap Configuration

Cache configuration can be provided via ConfigMap (future enhancement):

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: wva-metrics-config
data:
  cache_ttl: "30s"
  staleness_threshold: "2m"
  max_cache_size: "1000"
```

## Troubleshooting

### Common Issues

**Issue**: Metrics appear stale or missing
- **Solution**: Check metric timestamps; increase `StalenessThreshold` if needed
- **Debug**: Enable debug logging to see filtered metrics

**Issue**: High Prometheus query load
- **Solution**: Increase `CacheTTL` to reduce query frequency
- **Debug**: Monitor cache hit rates in logs

**Issue**: Inconsistent replica counts
- **Solution**: Verify pod labels match query selectors
- **Debug**: Check `PodVAMapper` pod discovery logs

### Debug Logging

Enable verbose logging to see metrics collection details:

```go
logger.V(logging.DEBUG).Info("Collected replica metrics",
    "modelID", modelID,
    "replicaCount", len(metrics),
    "cacheHits", cacheHits,
    "staleDrop", staleCount)
```

## Future Enhancements

1. **Additional Sources**: OpenMetrics, custom endpoints
2. **Advanced Caching**: Distributed cache, cache warming
3. **Query Optimization**: Batch queries, query result reuse
4. **Observability**: Metrics on cache performance, query latencies

## References

- [Saturation Analyzer Documentation](../saturation-analyzer.md)
- [PR #558: Metrics Cleanup Phase 1](https://github.com/llm-d-incubation/workload-variant-autoscaler/pull/558)
- [V2 Collector Source Code](../../internal/collector/v2/)
- [Saturation Metrics Implementation](../../internal/engines/saturation/metrics/)
