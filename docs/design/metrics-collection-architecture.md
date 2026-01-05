# Metrics Collection Architecture

## Overview

The Workload-Variant-Autoscaler (WVA) uses a pluggable metrics collection system to gather performance data from inference servers. This document describes the architecture, design decisions, and extension points for the metrics collection subsystem.

## Architecture

### Design Principles

1. **Pluggable Backend**: Support multiple metrics backends (Prometheus, EPP, custom sources) through a common interface
2. **Caching Strategy**: Reduce load on metrics backends through intelligent caching with configurable TTL
3. **Background Fetching**: Proactively fetch metrics to ensure fresh data is available during reconciliation
4. **Thread Safety**: Support concurrent access from controller reconciliation loops and background goroutines
5. **Testability**: Enable testing without real metrics backends through mock implementations

### Component Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Controller                                │
│                                                               │
│  ┌──────────────────────────────────────────────────────┐  │
│  │         VariantAutoscalingReconciler                  │  │
│  │                                                        │  │
│  │  Uses: MetricsCollector interface                    │  │
│  └────────────────────┬──────────────────────────────────┘  │
│                       │                                      │
└───────────────────────┼──────────────────────────────────────┘
                        │
                        ▼
        ┌───────────────────────────────┐
        │  MetricsCollector Interface   │
        │  (internal/interfaces)        │
        └───────────────┬───────────────┘
                        │
         ┌──────────────┴──────────────┐
         │                             │
         ▼                             ▼
┌────────────────────┐       ┌─────────────────────┐
│ PrometheusCollector│       │   EPP Collector     │
│ (internal/collector│       │  (future plugin)    │
│  /prometheus)      │       └─────────────────────┘
└────────┬───────────┘
         │
         ├─── MetricsCache (internal/collector/cache)
         │    ├─── MemoryCache: In-memory LRU cache
         │    └─── NoOpCache: Disabled caching
         │
         ├─── Background Fetcher (background_fetching.go)
         │    └─── Polling Executor: Periodic metric fetching
         │
         └─── Cache Operations (cache_operations.go)
              └─── Cache invalidation and management
```

## Core Components

### MetricsCollector Interface

Location: `internal/interfaces/metrics_collector.go`

The `MetricsCollector` interface defines the contract for all metrics collection backends:

```go
type MetricsCollector interface {
    // Validate metrics availability before attempting collection
    ValidateMetricsAvailability(ctx context.Context, modelName string, namespace string) MetricsValidationResult
    
    // Collect raw metrics for model-based optimization (proactive scaling)
    AddMetricsToOptStatus(ctx context.Context, va *VariantAutoscaling, deployment Deployment, acceleratorCostVal float64) (OptimizerMetrics, error)
    
    // Collect capacity metrics for saturation-based scaling (reactive scaling)
    CollectReplicaMetrics(ctx context.Context, modelID string, namespace string, deployments map[string]*Deployment, variantAutoscalings map[string]*VariantAutoscaling, variantCosts map[string]float64) ([]ReplicaMetrics, error)
}
```

**Key Methods:**

1. **ValidateMetricsAvailability**: Check if required metrics exist before attempting collection
2. **AddMetricsToOptStatus**: Collect metrics for model-based proactive scaling (arrival rate, token counts, latencies)
3. **CollectReplicaMetrics**: Collect per-replica capacity metrics for saturation-based reactive scaling (KV cache utilization, queue depth)

### Factory Pattern

Location: `internal/collector/factory.go`

The factory provides a centralized way to create collector instances:

```go
import "github.com/llm-d-incubation/workload-variant-autoscaler/internal/collector"

config := collector.Config{
    Type:        collector.CollectorTypePrometheus,
    PromAPI:     promAPI,
    CacheConfig: &config.CacheConfig{
        Enabled:         true,
        TTL:             30 * time.Second,
        CleanupInterval: 5 * time.Minute,
        FetchInterval:   15 * time.Second,
    },
}

metricsCollector, err := collector.NewMetricsCollector(config)
```

**Supported Collector Types:**
- `CollectorTypePrometheus`: Production-ready Prometheus backend
- `CollectorTypeEPP`: Placeholder for future EPP integration

### Prometheus Collector

Location: `internal/collector/prometheus/prometheus_collector.go`

The Prometheus implementation is the default and most mature collector backend.

**Key Features:**

1. **Cache Management**: 
   - Caches metric query results to reduce Prometheus load
   - Configurable TTL and cleanup intervals
   - Cache invalidation on deployment changes

2. **Background Fetching**:
   - Proactively fetches metrics in the background
   - Ensures fresh data available during reconciliation
   - Reduces reconciliation latency

3. **Thread Safety**:
   - Uses `sync.RWMutex` for concurrent access
   - Safe for use across multiple controller workers

4. **Tracking System**:
   - Tracks VariantAutoscaling resources for background fetching
   - Tracks models for replica metrics collection
   - Automatic cleanup when resources are deleted

### Metrics Cache

Location: `internal/collector/cache/`

The cache subsystem provides two implementations:

#### MemoryCache

In-memory cache with LRU eviction and TTL expiration:

```go
cache := cache.NewMemoryCache(
    30*time.Second,  // TTL
    5*time.Minute,   // Cleanup interval
)
```

**Features:**
- Thread-safe access with `sync.RWMutex`
- Automatic background cleanup of expired entries
- Prefix-based invalidation for bulk operations
- Age and expiration tracking

#### NoOpCache

Pass-through implementation for testing or when caching is disabled:

```go
cache := &cache.NoOpCache{}
```

**Cache Key Format:**

The cache uses structured keys to organize cached data:

```
For allocation metrics (model-based scaling):
    allocation:<namespace>/<vaName>

For replica metrics (saturation-based scaling):
    replica:<namespace>/<modelID>
```

### Background Fetching

Location: `internal/collector/prometheus/background_fetching.go`

The background fetcher proactively fetches metrics to ensure fresh data is available during reconciliation.

**Design:**

1. **Polling Executor**: Runs on a configurable interval (default: 15s)
2. **Tracked Resources**: Maintains a registry of VAs and models to fetch
3. **Fire-and-Forget**: Errors are logged but don't block the fetcher
4. **Cache Population**: Fetched metrics populate the cache for later use

**Tracking Operations:**

```go
// Start tracking a VariantAutoscaling for background fetching
collector.TrackVA(ctx, va, deployment, acceleratorCost)

// Start tracking a model for replica metrics fetching
collector.TrackModel(ctx, modelID, namespace, deployments, vas, costs)

// Stop tracking when resource is deleted
collector.UntrackVA(namespace, vaName)
collector.UntrackModel(modelID, namespace)
```

**Freshness Thresholds:**

The system defines freshness requirements for different metric types:

```go
type FreshnessThresholds struct {
    AllocationMetrics time.Duration  // Default: 30s
    ReplicaMetrics    time.Duration  // Default: 30s
}
```

## Configuration

### Cache Configuration

The cache can be configured via the `CacheConfig` struct:

```go
type CacheConfig struct {
    Enabled         bool                  // Enable/disable caching
    TTL             time.Duration         // Cache entry time-to-live
    CleanupInterval time.Duration         // Expired entry cleanup interval
    FetchInterval   time.Duration         // Background fetch interval (0 = disabled)
    FreshnessThresholds FreshnessThresholds // Freshness requirements
}
```

**Default Configuration:**

```go
CacheConfig{
    Enabled:         true,
    TTL:             30 * time.Second,
    CleanupInterval: 5 * time.Minute,
    FetchInterval:   15 * time.Second,
    FreshnessThresholds: FreshnessThresholds{
        AllocationMetrics: 30 * time.Second,
        ReplicaMetrics:    30 * time.Second,
    },
}
```

### Environment Variables

No environment variables are currently used for collector configuration. All configuration is done programmatically.

## Usage Patterns

### Controller Integration

The controller creates and uses the MetricsCollector during initialization:

```go
import (
    "github.com/llm-d-incubation/workload-variant-autoscaler/internal/collector"
    "github.com/llm-d-incubation/workload-variant-autoscaler/internal/interfaces"
)

// Create collector
metricsCollector, err := collector.NewMetricsCollector(collector.Config{
    Type:        collector.CollectorTypePrometheus,
    PromAPI:     promAPI,
    CacheConfig: nil, // Use defaults
})

// Set K8s client for pod ownership lookups
metricsCollector.(*prometheus.PrometheusCollector).SetK8sClient(mgr.GetClient())

// Use in reconciler
reconciler := &VariantAutoscalingReconciler{
    Client:           mgr.GetClient(),
    Scheme:           mgr.GetScheme(),
    Recorder:         mgr.GetEventRecorderFor("variantautoscaling-controller"),
    PromAPI:          promAPI,
    MetricsCollector: metricsCollector,
}
```

### Collecting Metrics

#### Model-Based Optimization (Proactive Scaling)

```go
// Validate metrics availability
result := metricsCollector.ValidateMetricsAvailability(ctx, modelName, namespace)
if !result.Available {
    return fmt.Errorf("metrics not available: %s", result.Message)
}

// Collect optimizer metrics
metrics, err := metricsCollector.AddMetricsToOptStatus(
    ctx, 
    variantAutoscaling, 
    deployment, 
    acceleratorCost,
)
if err != nil {
    return fmt.Errorf("failed to collect metrics: %w", err)
}

// Use metrics for optimization
allocation := buildAllocation(metrics)
```

#### Saturation-Based Scaling (Reactive Scaling)

```go
// Collect replica metrics for saturation analysis
replicaMetrics, err := metricsCollector.CollectReplicaMetrics(
    ctx,
    modelID,
    namespace,
    deployments,
    variantAutoscalings,
    variantCosts,
)
if err != nil {
    return fmt.Errorf("failed to collect replica metrics: %w", err)
}

// Use metrics for saturation analysis
decision := saturationAnalyzer.AnalyzeSaturation(replicaMetrics)
```

### Background Fetching

The Prometheus collector supports background metric fetching to improve reconciliation latency:

```go
pc := metricsCollector.(*prometheus.PrometheusCollector)

// Start background fetching
pc.StartBackgroundFetching(ctx)

// Track a VariantAutoscaling for background fetching
pc.TrackVA(ctx, va, deployment, acceleratorCost)

// Track a model for replica metrics
pc.TrackModel(ctx, modelID, namespace, deployments, vas, costs)

// Later: stop tracking when resource is deleted
pc.UntrackVA(namespace, vaName)
pc.UntrackModel(modelID, namespace)

// Stop background fetching on shutdown
pc.StopBackgroundFetching()
```

## Extension Points

### Adding a New Collector Backend

To add support for a new metrics backend (e.g., EPP, OpenMetrics):

1. **Implement the MetricsCollector interface:**

```go
package mybackend

import "github.com/llm-d-incubation/workload-variant-autoscaler/internal/interfaces"

type MyCollector struct {
    // Your fields
}

func (c *MyCollector) ValidateMetricsAvailability(ctx context.Context, modelName string, namespace string) interfaces.MetricsValidationResult {
    // Implementation
}

func (c *MyCollector) AddMetricsToOptStatus(...) (interfaces.OptimizerMetrics, error) {
    // Implementation
}

func (c *MyCollector) CollectReplicaMetrics(...) ([]interfaces.ReplicaMetrics, error) {
    // Implementation
}
```

2. **Register in factory:**

```go
// In internal/collector/factory.go
const CollectorTypeMyBackend CollectorType = "mybackend"

func NewMetricsCollector(config Config) (interfaces.MetricsCollector, error) {
    switch config.Type {
    case CollectorTypeMyBackend:
        return mybackend.NewMyCollector(config), nil
    // ... existing cases
    }
}
```

3. **Add tests:**

```go
// In your package
func TestMyCollector(t *testing.T) {
    // Test implementation
}
```

### Cache Strategies

Different cache strategies can be implemented by creating new `MetricsCache` implementations:

```go
type CustomCache struct {
    // Your fields
}

func (c *CustomCache) Get(key CacheKey) (*CachedMetrics, bool) {
    // Implementation
}

func (c *CustomCache) Set(key CacheKey, data interface{}, ttl time.Duration) {
    // Implementation
}

// ... other methods
```

## Performance Considerations

### Cache Hit Rates

Monitor cache effectiveness through controller logs:

```
Metrics cache enabled TTL=30s cleanupInterval=5m0s
Using cached allocation metrics age=5s
Cache miss for allocation metrics, fetching from Prometheus
```

**Expected Hit Rates:**
- **High traffic**: 70-90% (reconciliations happen frequently within TTL window)
- **Low traffic**: 30-50% (reconciliations spread out, cache expires)

### Background Fetching Impact

Background fetching reduces reconciliation latency at the cost of increased Prometheus query load:

| Configuration | Reconciliation Latency | Prometheus Query Rate |
|---------------|------------------------|----------------------|
| No background fetching | 500-1000ms | ~60 queries/min |
| 15s fetch interval | 50-100ms | ~240 queries/min |
| 30s fetch interval | 100-200ms | ~120 queries/min |

**Recommendation**: Use 15-30s fetch intervals for production workloads.

### Memory Usage

Cache memory usage scales with the number of tracked resources:

```
Memory per cached entry: ~2-5 KB
Typical deployment: 10-100 VAs = 20-500 KB
Large deployment: 1000+ VAs = 2-5 MB
```

Cleanup intervals prevent unbounded growth.

## Troubleshooting

### Cache Issues

**Problem**: Stale metrics despite fresh data in Prometheus

**Solution**: Check cache TTL configuration and invalidation logic:

```bash
# Check controller logs for cache operations
kubectl logs -n workload-variant-autoscaler-system deployment/workload-variant-autoscaler-controller-manager | grep -i cache
```

**Problem**: High Prometheus query load

**Solution**: Increase cache TTL or disable background fetching:

```go
CacheConfig{
    TTL:           60 * time.Second,  // Increase from 30s
    FetchInterval: 0,                  // Disable background fetching
}
```

### Background Fetching Issues

**Problem**: Background fetching not working

**Solution**: Verify:
1. `FetchInterval` is non-zero
2. `StartBackgroundFetching()` was called
3. Resources are tracked via `TrackVA()` or `TrackModel()`

```bash
# Check for background fetching logs
kubectl logs -n workload-variant-autoscaler-system deployment/workload-variant-autoscaler-controller-manager | grep "background fetching"
```

### Metric Collection Failures

**Problem**: `CollectReplicaMetrics` returns empty results

**Solution**: Check:
1. Prometheus has metrics for the model/namespace
2. ServiceMonitor is correctly configured
3. Metric names match expectations

```promql
# Verify metrics exist in Prometheus
vllm:gpu_cache_usage_perc{namespace="your-namespace"}
vllm:num_requests_waiting{namespace="your-namespace"}
```

## Future Enhancements

### Planned Features

1. **EPP Direct Integration**: Native support for EPP metrics backend
2. **Distributed Caching**: Redis/Memcached support for multi-controller deployments
3. **Adaptive TTL**: Dynamic TTL adjustment based on metric volatility
4. **Metric Aggregation**: Pre-computed aggregates for common queries
5. **Health Monitoring**: Collector health metrics and alerting

### Extensibility Roadmap

- [ ] OpenMetrics support
- [ ] Custom metric backends via plugins
- [ ] Metric preprocessing pipelines
- [ ] Query optimization and batching
- [ ] Multi-region metric federation

## References

### Code Locations

- **Interface**: `internal/interfaces/metrics_collector.go`
- **Factory**: `internal/collector/factory.go`
- **Prometheus Collector**: `internal/collector/prometheus/prometheus_collector.go`
- **Cache**: `internal/collector/cache/`
- **Background Fetching**: `internal/collector/prometheus/background_fetching.go`

### Related Documentation

- [Saturation Analyzer](../saturation-analyzer.md)
- [Metrics and Health Monitoring](../metrics-health-monitoring.md)
- [Prometheus Integration](../integrations/prometheus.md)
- [Development Guide](../developer-guide/development.md)

### Metrics Reference

See [Prometheus Integration](../integrations/prometheus.md) for complete metric definitions.
