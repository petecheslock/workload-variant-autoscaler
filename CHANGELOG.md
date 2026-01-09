# Changelog

All notable changes to the Workload-Variant-Autoscaler project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- New comprehensive documentation for V2 metrics collection architecture
- Developer guide section on metrics collection v2 system

### Changed
- Updated documentation structure references to reflect actual codebase layout

## [0.4.0] - 2026-01-09

### 🚀 Major Changes

#### Metrics Collection V2 Now Default ([#558](https://github.com/llm-d-incubation/workload-variant-autoscaler/pull/558))

The V2 metrics collection architecture is now the default and only metrics collection system.

**Breaking Changes:**
- **Removed** legacy v1 `MetricsCollector` interface (`internal/collector/collector.go`)
- **Removed** legacy Prometheus collector implementation (`internal/collector/prometheus/prometheus_collector.go`)
- **Removed** `COLLECTOR_V2` environment variable - no longer needed

**What This Means:**
- All metrics collection now uses the v2 collector infrastructure (`internal/collector/v2/`)
- Cleaner, more maintainable codebase
- Better testability with `MetricsSource` abstraction
- Improved caching and staleness handling

### Changed

#### Architecture Improvements
- Simplified engine initialization - no longer requires legacy `MetricsCollector`
- Controller reconciler no longer depends on `MetricsCollector` or `PromAPI`
- Saturation engine exclusively uses `ReplicaMetricsCollectorV2`
- Removed conditional v1/v2 collector logic throughout the codebase

#### Caching Improvements
- **Removed** manual cache invalidation on scaling events
- Automatic staleness filtering handles cache freshness (default: 2-minute threshold)
- Metric freshness is checked automatically during collection

### Removed

- `internal/collector/collector.go` - Legacy MetricsCollector interface
- `internal/collector/prometheus/prometheus_collector.go` - Legacy Prometheus implementation
- `COLLECTOR_V2` environment variable support
- `UseCollectorV2()` helper function (no longer needed)
- Manual cache invalidation code in saturation engine
- `MetricsCollector` field from `VariantAutoscalingReconciler`
- `PromAPI` field from `VariantAutoscalingReconciler`

### Fixed
- Removed unused `strings` import in `cmd/main.go`
- Cleaned up test fixtures to use v2 collector consistently

### Developer Experience

#### Migration Guide

**Before (v0.3.x):**
```go
// Old way - v1 collector
metricsCollector, err := collector.NewMetricsCollector(collector.Config{
    Type:        collector.CollectorTypePrometheus,
    PromAPI:     promAPI,
    CacheConfig: cacheConfig,
})

reconciler := &controller.VariantAutoscalingReconciler{
    Client:           mgr.GetClient(),
    MetricsCollector: metricsCollector,
    PromAPI:          promAPI,
}
```

**After (v0.4.0):**
```go
// New way - v2 collector
sourceRegistry := collectorv2.NewSourceRegistry()
promSource := collectorv2.NewPrometheusSource(ctx, promAPI, 
    collectorv2.DefaultPrometheusSourceConfig())
sourceRegistry.Register("prometheus", promSource)

reconciler := &controller.VariantAutoscalingReconciler{
    Client: mgr.GetClient(),
    Scheme: mgr.GetScheme(),
}

engine := saturation.NewEngine(
    mgr.GetClient(),
    mgr.GetScheme(),
    recorder,
    sourceRegistry,
)
```

#### Testing Updates

**Before:**
```go
mockPromAPI := &MockPromAPI{}
metricsCollector := collector.NewPrometheusCollector(mockPromAPI)
engine := NewEngine(client, scheme, recorder, metricsCollector, registry)
```

**After:**
```go
sourceRegistry := collectorv2.NewSourceRegistry()
promSource := collectorv2.NewPrometheusSource(ctx, mockPromAPI, config)
sourceRegistry.Register("prometheus", promSource)
engine := NewEngine(client, scheme, recorder, sourceRegistry)
```

### Documentation

- Added comprehensive [Metrics Collection V2 Architecture](docs/developer-guide/metrics-collection-v2.md) guide
- Updated [Saturation Analyzer](docs/saturation-analyzer.md) with correct file paths
- Updated [Developer Guide](docs/developer-guide/development.md) with v2 collector information
- Updated project structure documentation to reflect actual codebase layout

### Technical Details

#### V2 Collector Architecture Benefits

1. **Better Separation of Concerns**
   - Source layer: Raw metric fetching and caching
   - Collector layer: Domain-specific aggregation
   - Analysis layer: Business logic

2. **Improved Testability**
   - `NoOpSource` for testing without metrics
   - `MetricsSource` interface for mocking
   - Isolated unit testing

3. **Enhanced Performance**
   - Built-in caching with configurable TTL (default: 30s)
   - Automatic staleness filtering (default: 2m)
   - Reduced Prometheus query load

4. **Better Extensibility**
   - Query template system for parameterized queries
   - Registry pattern for multiple metrics sources
   - Easy to add new backends (OpenMetrics, custom endpoints)

## [0.3.x] - Previous Releases

See GitHub releases for previous version history.

---

## Migration Notes

### From 0.3.x to 0.4.0

**Required Changes:**
1. Remove any references to `COLLECTOR_V2` environment variable
2. Update custom code to use v2 collector API if you have extensions
3. Update tests to use `collectorv2` package imports

**No Action Required:**
- If using standard deployment methods (Helm, kubectl), no changes needed
- WVA automatically uses v2 collector

**Verification:**
Check logs for "Initializing v2 collector" message:
```
{"level":"info","ts":"2026-01-09T18:03:15Z","msg":"Initializing v2 collector"}
```

---

## Links

- [GitHub Repository](https://github.com/llm-d-incubation/workload-variant-autoscaler)
- [Documentation](docs/README.md)
- [Contributing Guide](CONTRIBUTING.md)
