# Collector V2

Package `collector/v2` provides a modular metrics collection system with per-source query registries.

## Quick Start

### Basic Usage

```go
import (
    collectorv2 "github.com/llm-d-incubation/workload-variant-autoscaler/internal/collector/v2"
    promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// 1. Create a Prometheus source
config := collectorv2.DefaultPrometheusSourceConfig()
promSource := collectorv2.NewPrometheusSource(ctx, promAPI, config)

// 2. Register queries
promSource.QueryList().Register(collectorv2.QueryTemplate{
    Name:        "kv_cache_usage",
    Type:        collectorv2.QueryTypePromQL,
    Template:    `max by (pod) (vllm:kv_cache_usage_perc{namespace="{{.namespace}}"})`,
    Params:      []string{"namespace"},
    Description: "KV cache utilization per pod",
})

// 3. Execute queries and cache results
results, err := promSource.Refresh(ctx, collectorv2.RefreshSpec{
    Queries: []string{"kv_cache_usage"},
    Params: map[string]string{"namespace": "default"},
})

// 4. Retrieve cached values
cached := promSource.Get("kv_cache_usage", map[string]string{"namespace": "default"})
if cached != nil && !cached.IsExpired() {
    for _, value := range cached.Result.Values {
        fmt.Printf("Pod %s: %.2f%%\n", value.Labels["pod"], value.Value)
    }
}
```

## Key Concepts

### Metrics Source

A `MetricsSource` is a backend that provides metrics (Prometheus, pod scraping, etc.):

```go
type MetricsSource interface {
    QueryList() *QueryList
    Refresh(ctx context.Context, spec RefreshSpec) (map[string]*MetricResult, error)
    Get(queryName string, params map[string]string) *CachedValue
}
```

### Query Templates

Define parameterized queries with template syntax:

```go
type QueryTemplate struct {
    Name        string      // Unique identifier
    Type        QueryType   // "metric" or "promql"
    Template    string      // Query with {{.param}} placeholders
    Params      []string    // Required parameters
    Description string      // Documentation
}
```

**Query Types:**
- `QueryTypeMetricName` - Simple metric name (e.g., `"vllm:kv_cache_usage_perc"`)
- `QueryTypePromQL` - Full PromQL with parameters

**Template Parameters:**

Common parameters defined as constants:
```go
const (
    ParamNamespace = "namespace"
    ParamModelID   = "modelID"
    ParamPodFilter = "podFilter"  // Optional regex filter
)
```

### Source Registry

Manage multiple sources globally:

```go
registry := collectorv2.NewSourceRegistry()
registry.Register("prometheus", promSource)
registry.Register("epp", eppSource)

// Retrieve a source
source := registry.Get("prometheus")
```

## Architecture

### Query Registration Flow

```
Controller Startup
    ↓
Create PrometheusSource
    ↓
Register Queries ────────→ QueryList
    ↓                         ↓
Register in Registry    Store Templates
    ↓
Ready for Queries
```

### Query Execution Flow

```
Client calls Refresh()
    ↓
For each query:
    ↓
Render Template with Params
    ↓
Execute PromQL Query
    ↓
Parse Results
    ↓
Cache with TTL ──────→ Cache
    ↓
Return Results

Later: Client calls Get()
    ↓
Lookup in Cache
    ↓
Return if not expired
```

### Caching Strategy

```go
type Cache struct {
    // Key: {queryName}:{param1=value1,param2=value2}
    // Value: CachedValue with expiration
}

type CachedValue struct {
    Result    *MetricResult
    CachedAt  time.Time
    ExpiresAt time.Time
}
```

**Features:**
- Thread-safe with RW locks
- Automatic expiration cleanup
- Per-query TTL configuration
- Configurable default TTL

## Components

### PrometheusSource

Main implementation for Prometheus backend:

```go
type PrometheusSource struct {
    api      promv1.API
    registry *QueryList
    config   PrometheusSourceConfig
    cache    *Cache
}
```

**Configuration:**
```go
type PrometheusSourceConfig struct {
    DefaultTTL   time.Duration  // Default: 30s
    QueryTimeout time.Duration  // Default: 10s
}
```

**Methods:**

- `QueryList()` - Access query registry for registration
- `Refresh(ctx, spec)` - Execute queries and update cache
- `Get(queryName, params)` - Retrieve cached result

### PodVAMapper

Tracks pod-to-VariantAutoscaling relationships:

```go
mapper := collectorv2.NewPodVAMapper()

// Register VA
mapper.Register("llama-8b-autoscaler", "default", "llama-deployment")

// Query VA for pod
vaName := mapper.GetVAForPod("llama-deployment-abc123-xyz")
// Returns: "llama-8b-autoscaler"
```

**Use Case:** Filter metrics by VariantAutoscaling resource.

### Query Template Rendering

Templates use Go `text/template` syntax:

```go
template := `max by (pod) ({{.metric}}{namespace="{{.namespace}}",model_id="{{.modelID}}"})`

params := map[string]string{
    "metric":    "vllm:kv_cache_usage_perc",
    "namespace": "llm-inference",
    "modelID":   "llama-8b",
}

// Renders to:
// max by (pod) (vllm:kv_cache_usage_perc{namespace="llm-inference",model_id="llama-8b"})
```

**Safety:** Values are automatically escaped for PromQL.

## Usage Patterns

### Pattern 1: Single Query

```go
// Register once at startup
promSource.QueryList().Register(QueryTemplate{
    Name:     "queue_depth",
    Type:     QueryTypePromQL,
    Template: `vllm:num_requests_waiting{namespace="{{.namespace}}"}`,
    Params:   []string{"namespace"},
})

// Execute in reconciliation loop
results, _ := promSource.Refresh(ctx, RefreshSpec{
    Queries: []string{"queue_depth"},
    Params:  map[string]string{"namespace": "default"},
})
```

### Pattern 2: Batch Queries

```go
// Register multiple queries
queries := []QueryTemplate{
    {Name: "kv_cache", Type: QueryTypePromQL, Template: "...", Params: []string{"namespace"}},
    {Name: "queue_depth", Type: QueryTypePromQL, Template: "...", Params: []string{"namespace"}},
    {Name: "gpu_util", Type: QueryTypePromQL, Template: "...", Params: []string{"namespace"}},
}
for _, q := range queries {
    promSource.QueryList().Register(q)
}

// Execute all at once
results, _ := promSource.Refresh(ctx, RefreshSpec{
    Queries: []string{"kv_cache", "queue_depth", "gpu_util"},
    Params:  map[string]string{"namespace": "default"},
})
```

### Pattern 3: Refresh All Registered

```go
// Leave Queries empty to refresh all
results, _ := promSource.Refresh(ctx, RefreshSpec{
    Params: map[string]string{"namespace": "default"},
})
```

### Pattern 4: Cached Access

```go
// Refresh once
promSource.Refresh(ctx, RefreshSpec{
    Queries: []string{"kv_cache"},
    Params:  map[string]string{"namespace": "default"},
})

// Multiple fast cached reads (within TTL)
for _, pod := range pods {
    cached := promSource.Get("kv_cache", map[string]string{"namespace": "default"})
    if cached != nil && !cached.IsExpired() {
        // Use cached.Result
    }
}
```

## Configuration

### Environment Variables

Enable collector v2 in your controller:

```bash
export COLLECTOR_V2=true
```

### Tuning Cache TTL

```go
config := collectorv2.PrometheusSourceConfig{
    DefaultTTL:   60 * time.Second,  // Longer cache for stable metrics
    QueryTimeout: 15 * time.Second,  // Longer timeout for complex queries
}
source := collectorv2.NewPrometheusSource(ctx, promAPI, config)
```

## Testing

### Unit Tests

Test query registration and execution:

```go
func TestPrometheusSource(t *testing.T) {
    source := collectorv2.NewPrometheusSource(ctx, mockAPI, collectorv2.DefaultPrometheusSourceConfig())
    
    source.QueryList().Register(collectorv2.QueryTemplate{
        Name:     "test_metric",
        Type:     collectorv2.QueryTypePromQL,
        Template: `up{namespace="{{.namespace}}"}`,
        Params:   []string{"namespace"},
    })
    
    results, err := source.Refresh(ctx, collectorv2.RefreshSpec{
        Queries: []string{"test_metric"},
        Params:  map[string]string{"namespace": "default"},
    })
    
    require.NoError(t, err)
    assert.NotNil(t, results["test_metric"])
}
```

### Integration Tests

See `prometheus_source_test.go` for comprehensive examples.

## Migration from Legacy Collector

### Before (Legacy)

```go
// Global collector with implicit queries
collector := prometheus.NewCollector(promAPI)
metrics := collector.CollectSaturationMetricsForReplica(ctx, va, deployment, pod)
```

### After (Collector V2)

```go
// Per-source explicit queries
promSource := collectorv2.NewPrometheusSource(ctx, promAPI, config)
promSource.QueryList().Register(...)  // Explicit registration

results, _ := promSource.Refresh(ctx, RefreshSpec{...})
cached := promSource.Get("query_name", params)
```

### Key Differences

| Aspect | Legacy | V2 |
|--------|--------|-----|
| **Query Management** | Implicit, hardcoded | Explicit registration |
| **Cache Strategy** | Global shared cache | Per-source isolated |
| **Extensibility** | Single source only | Multiple sources |
| **Testing** | Coupled to Prometheus | Interface-based mocking |

## Performance

### Benchmarks

Typical performance characteristics:

- **Query Registration:** O(1) with mutex
- **Cache Lookup:** O(1) with RW lock
- **Refresh (single query):** ~10-50ms (network-bound)
- **Refresh (batch 10 queries):** ~50-200ms (parallel execution)

### Memory Usage

- **Per Query Template:** ~200 bytes
- **Per Cached Result:** Varies by metric (typically 1-10 KB)
- **Cache Cleanup:** Automatic background goroutine

## Limitations

### Current Limitations

1. **No Query Result Aggregation:** Each query returns independent results
2. **Single Time Point:** No time-series support (use PromQL `rate()` etc.)
3. **No Cross-Source Queries:** Can't join metrics from different sources

### Planned Improvements

1. Result aggregation across sources
2. Time-series query support
3. Query result transformation pipeline
4. Metrics federation for multi-cluster

## Examples

See the following files for complete examples:

- `prometheus_source_test.go` - Comprehensive test suite
- `../../engines/saturation/metrics/register.go` - Production query registration
- `../../engines/saturation/engine.go` - Integration with saturation engine

## API Reference

### Types

```go
type MetricsSource interface { ... }
type QueryTemplate struct { ... }
type QueryList struct { ... }
type SourceRegistry struct { ... }
type PrometheusSource struct { ... }
type PodVAMapper struct { ... }
type Cache struct { ... }
type CachedValue struct { ... }
type MetricResult struct { ... }
type MetricValue struct { ... }
type RefreshSpec struct { ... }
```

### Functions

```go
// Constructor functions
func NewSourceRegistry() *SourceRegistry
func NewPrometheusSource(ctx context.Context, api promv1.API, config PrometheusSourceConfig) *PrometheusSource
func DefaultPrometheusSourceConfig() PrometheusSourceConfig
func NewPodVAMapper() *PodVAMapper

// Query management
func (q *QueryList) Register(template QueryTemplate) error
func (q *QueryList) MustRegister(template QueryTemplate)
func (q *QueryList) Get(name string) (QueryTemplate, bool)
func (q *QueryList) List() []string

// Registry operations
func (r *SourceRegistry) Register(name string, source MetricsSource) error
func (r *SourceRegistry) Get(name string) MetricsSource
func (r *SourceRegistry) List() []string

// Source operations
func (p *PrometheusSource) QueryList() *QueryList
func (p *PrometheusSource) Refresh(ctx context.Context, spec RefreshSpec) (map[string]*MetricResult, error)
func (p *PrometheusSource) Get(queryName string, params map[string]string) *CachedValue
```

## Further Reading

- [Collector Architecture Design Doc](../../docs/design/collector-architecture.md)
- [Prometheus Integration](../../docs/integrations/prometheus.md)
- [Saturation Engine Documentation](../../docs/saturation-analyzer.md)
