# Metrics Collector Architecture

This document describes the metrics collection architecture in the Workload-Variant-Autoscaler (WVA), including both the legacy collector and the new v2 collector design.

## Overview

WVA's metrics collection system gathers performance data from various sources (primarily Prometheus) to inform autoscaling decisions. The system has evolved from a monolithic collector to a modular, source-based architecture (collector v2).

## Architecture Evolution

### Legacy Collector (v1)

The original collector implementation (`internal/collector`) provides:

- Direct Prometheus API integration
- Background fetching for specific metrics
- Cache management with configurable TTL
- Saturation-specific metric collection

**Key Components:**
- `collector.go` - Main collector interface and deprecated compatibility functions
- `prometheus/` - Prometheus-specific implementation with background fetching
- `cache/` - In-memory caching layer with expiration
- `config/` - Configuration management

**Limitations:**
- Global query list shared across all use cases
- Tight coupling between query definition and execution
- Limited extensibility for new metric sources

### Collector V2 (Current)

The v2 collector (`internal/collector/v2`) introduces a modular, source-based architecture behind the `COLLECTOR_V2` feature flag.

**Key Improvements:**
- Per-source query registries
- Pluggable metrics sources (Prometheus, pod scraping, EPP)
- Unified caching strategy
- Template-based query system
- Better separation of concerns

## Collector V2 Architecture

### Core Concepts

#### 1. Metrics Source

A `MetricsSource` represents a backend system that provides metrics (e.g., Prometheus, direct pod scraping).

```go
type MetricsSource interface {
    // QueryList returns the query registry for this source
    QueryList() *QueryList
    
    // Refresh executes queries and updates the cache
    Refresh(ctx context.Context, spec RefreshSpec) (map[string]*MetricResult, error)
    
    // Get retrieves a cached value for a query
    Get(queryName string, params map[string]string) *CachedValue
}
```

Each source:
- Maintains its own query registry
- Manages its own cache with configurable TTL
- Executes queries specific to its backend

#### 2. Query Registry

Each metrics source has a `QueryList` that manages query templates:

```go
type QueryTemplate struct {
    Name        string      // Unique identifier (e.g., "kv_cache_usage")
    Type        QueryType   // "metric" or "promql"
    Template    string      // Query with {{.ParamName}} placeholders
    Params      []string    // Required parameter names
    Description string      // Documentation
}
```

**Query Types:**
- `QueryTypeMetricName` - Simple metric name for basic backends
- `QueryTypePromQL` - Full PromQL expression with template support

**Example Registration:**

```go
source.QueryList().Register(QueryTemplate{
    Name:        "kv_cache_usage",
    Type:        QueryTypePromQL,
    Template:    `max by (pod) (vllm:kv_cache_usage_perc{namespace="{{.namespace}}"})`,
    Params:      []string{"namespace"},
    Description: "KV cache utilization percentage per pod",
})
```

#### 3. Source Registry

The `SourceRegistry` manages multiple metrics sources:

```go
registry := collectorv2.NewSourceRegistry()
registry.Register("prometheus", promSource)
registry.Register("epp", eppSource)
```

This enables:
- Multiple concurrent sources
- Source-specific query optimization
- Isolated caching per source

#### 4. Caching Strategy

Each source maintains an internal cache with:
- Configurable TTL per query result
- Automatic cleanup of expired entries
- Thread-safe concurrent access
- Cache key includes query name + parameters

```go
type CachedValue struct {
    Result     *MetricResult
    CachedAt   time.Time
    ExpiresAt  time.Time
}
```

### Prometheus Source Implementation

The `PrometheusSource` is the primary implementation:

**Configuration:**

```go
config := collectorv2.DefaultPrometheusSourceConfig()
// DefaultTTL: 30 seconds
// QueryTimeout: 10 seconds

source := collectorv2.NewPrometheusSource(ctx, promAPI, config)
```

**Query Execution Flow:**

1. Client calls `Refresh()` with optional query names
2. Source executes specified queries (or all registered queries)
3. Results are parsed and cached with TTL
4. Clients retrieve cached values via `Get()`

**Template Parameter Substitution:**

Templates support Go text/template syntax:
```promql
max by (pod) ({{.metricName}}{namespace="{{.namespace}}",modelID="{{.modelID}}"})
```

Parameters are validated and escaped for PromQL safety.

### Pod-VA Mapper

The `PodVAMapper` tracks relationships between pods and VariantAutoscaling resources:

```go
mapper := collectorv2.NewPodVAMapper()

// Register a VA with its target deployment
mapper.Register("my-va", "my-namespace", "my-deployment")

// Query VA for a pod
vaName := mapper.GetVAForPod("pod-xyz-123")
```

This enables:
- Per-VA metric filtering
- Dynamic pod tracking
- Efficient VA-to-pod lookups

## Integration Points

### Saturation Engine Integration

The saturation engine uses collector v2 when `COLLECTOR_V2=true`:

```go
if engine.UseCollectorV2() {
    // Use v2 replica metrics collector
    metrics := engine.ReplicaMetricsCollectorV2.CollectForPod(ctx, podName, vaName)
} else {
    // Fall back to legacy collector
    metrics := engine.MetricsCollector.CollectSaturationMetricsForReplica(ctx, ...)
}
```

### Query Registration at Startup

Queries are registered during controller initialization:

```go
// cmd/main.go
promSource := collectorv2.NewPrometheusSource(ctx, promAPI, config)

// Register queries specific to saturation-based scaling
saturationmetrics.RegisterReplicaQueries(promSource.QueryList())

// Register in global registry
sourceRegistry.Register("prometheus", promSource)
```

## Migration Strategy

### Feature Flag

The `COLLECTOR_V2` environment variable enables the new collector:

```bash
export COLLECTOR_V2=true
```

When disabled, WVA uses the legacy collector for backward compatibility.

### Coexistence

Both collectors can coexist during migration:
- Legacy collector handles existing queries
- V2 collector handles new queries
- Gradual migration path per metric

### Rollout Plan

1. **Phase 1** (Current): V2 available behind feature flag
2. **Phase 2**: Default to V2, keep legacy as fallback
3. **Phase 3**: Remove legacy collector code

## Performance Considerations

### Caching Strategy

- **Default TTL**: 30 seconds (configurable per source)
- **Cleanup Interval**: 1 second background cleanup
- **Cache Key**: `{queryName}:{param1=value1,param2=value2}`

### Query Optimization

- Batch query execution in single `Refresh()` call
- Parallel query execution within a source
- Deduplicated cache lookups

### Memory Management

- Automatic expiration of stale cache entries
- No unbounded cache growth
- Thread-safe with RW locks

## Future Extensions

### Planned Features

1. **EPP Source**: Direct pod endpoint scraping without Prometheus
2. **Direct API Source**: Query inference server APIs directly
3. **Multi-cluster Support**: Aggregate metrics across clusters
4. **Query Result Aggregation**: Combine results from multiple sources

### Extensibility Points

To add a new metrics source:

1. Implement the `MetricsSource` interface
2. Define source-specific query templates
3. Register in `SourceRegistry`
4. Update engine to use new source

Example:

```go
type CustomSource struct {
    registry *QueryList
    cache    *Cache
}

func (c *CustomSource) QueryList() *QueryList { return c.registry }
func (c *CustomSource) Refresh(ctx context.Context, spec RefreshSpec) (map[string]*MetricResult, error) {
    // Custom query execution logic
}
func (c *CustomSource) Get(queryName string, params map[string]string) *CachedValue {
    // Custom cache lookup logic
}
```

## Testing

### Unit Tests

Each component has comprehensive unit tests:
- `prometheus_source_test.go` - Prometheus integration
- `cache_test.go` - Cache behavior
- `pod_va_mapper_test.go` - Pod tracking
- `query_template_test.go` - Template rendering

### Integration Tests

Saturation engine tests validate both collectors:
- `engine_test.go` includes scenarios with and without `COLLECTOR_V2`

### Test Coverage

Run tests for collector v2:

```bash
go test ./internal/collector/v2/... -v -cover
```

## Configuration

### Environment Variables

- `COLLECTOR_V2` - Enable collector v2 (default: `false`)

### Prometheus Source Config

```go
type PrometheusSourceConfig struct {
    DefaultTTL   time.Duration  // Cache TTL (default: 30s)
    QueryTimeout time.Duration  // Query timeout (default: 10s)
}
```

## Troubleshooting

### Common Issues

**Queries not executing:**
- Verify query is registered in source's `QueryList`
- Check `COLLECTOR_V2` environment variable
- Confirm source is registered in `SourceRegistry`

**Cache misses:**
- Verify parameter names match query template
- Check TTL hasn't expired
- Ensure `Refresh()` called before `Get()`

**Missing metrics:**
- Verify Prometheus has the metric
- Check namespace and label filters
- Review query template syntax

### Debug Logging

Enable debug logging to trace collector operations:

```bash
export LOG_LEVEL=debug
```

Look for log entries:
- `"Refreshing queries"` - Query execution start
- `"Query result cached"` - Successful cache update
- `"Cache hit"` / `"Cache miss"` - Cache lookups

## References

- [Metrics Collection Interface](../../internal/interfaces/metrics_collector.go)
- [Prometheus Integration](../integrations/prometheus.md)
- [Saturation Analyzer](../saturation-analyzer.md)
- [Package Documentation](../../internal/collector/v2/doc.go)

## Related Components

- **Saturation Engine** - Primary consumer of collector v2
- **Actuator** - Emits metrics back to Prometheus
- **Prometheus Adapter** - External metrics for HPA/KEDA
