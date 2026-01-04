# WVA Engine Architecture

## Overview

The Workload Variant Autoscaler (WVA) v0.4+ introduces an **engine-based architecture** that provides flexible scaling strategies through pluggable engines. This design separates concerns between collection, analysis, optimization, and actuation while supporting multiple scaling approaches.

## Architecture Components

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Kubernetes API Server                         │
└───────────────────────┬─────────────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────────────┐
│                   WVA Controller                                 │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  Reconciler (variantautoscaling_controller.go)           │   │
│  │  - Watches VariantAutoscaling CRDs                       │   │
│  │  - Delegates to Engine based on mode                     │   │
│  └────────────────────┬─────────────────────────────────────┘   │
└─────────────────────┬─┴─────────────────────────────────────────┘
                      │
         ┌────────────┴────────────┐
         │                         │
┌────────▼────────┐   ┌────────────▼──────────┐
│ Saturation      │   │ Model Engine          │
│ Engine          │   │ (Future)              │
│ (Default)       │   │                       │
└────────┬────────┘   └───────────────────────┘
         │
    ┌────┴────┐
    │         │
┌───▼──┐  ┌──▼─────┐
│Coll. │  │Analyzer│
└──┬───┘  └───┬────┘
   │          │
┌──▼──────────▼────┐
│   Prometheus     │
│   (vLLM Metrics) │
└──────────────────┘
```

## Engine Types

WVA supports multiple scaling engines with different optimization strategies:

### 1. Saturation Engine (Default)

**Location**: `internal/engines/saturation/`

**Purpose**: Reactive saturation-based scaling using live vLLM metrics.

**Key Features**:
- ✅ No offline profiling required
- ✅ Monitors KV-cache utilization and request queue depth
- ✅ Per-variant scaling decisions with cost awareness
- ✅ Safe scale-down with worst-case simulation
- ✅ Architecture-agnostic (works with all model types)

**Components**:

#### Metrics Collector (`internal/collector/`)
- **Prometheus Collector** (`prometheus/prometheus_collector.go`): Queries Prometheus API for vLLM metrics
- **Saturation Metrics** (`prometheus/saturation_metrics.go`): Collects KV-cache and queue metrics
- **Caching** (`cache/`): In-memory cache with TTL to reduce Prometheus API load
- **Background Fetching** (`prometheus/background_fetching.go`): Proactive metric collection

#### Saturation Analyzer (`internal/saturation/`)
- **Analyzer** (`analyzer.go`): Core saturation analysis logic
- Calculates spare capacity across replicas
- Determines scale-up triggers
- Simulates scale-down safety
- Generates per-variant scaling decisions

#### Executor (`internal/engines/executor/`)
- **Polling Executor** (`polling.go`): Fixed-interval task execution
- Configurable polling interval (default: 30s)
- Retry with exponential backoff
- Thread-safe concurrent execution

**Data Flow**:
```
1. Executor triggers optimization cycle (every 30s)
2. Collector queries Prometheus for vLLM metrics
   - vllm:kv_cache_usage_perc (KV-cache utilization)
   - vllm:num_requests_waiting (queue depth)
3. Analyzer processes metrics per variant
   - Identifies saturated vs non-saturated replicas
   - Calculates average spare capacity
   - Determines scale-up/down decisions
4. Engine updates decision cache
5. Controller reads decision from cache
6. Actuator emits metrics to Prometheus
7. HPA/KEDA reads metrics and scales deployment
```

**Configuration**:
- **ConfigMap**: `capacity-scaling-config` (see [Saturation Scaling Config](../saturation-scaling-config.md))
- **Parameters**:
  - `kvCacheThreshold`: Scale-up when utilization ≥ threshold (default: 0.80)
  - `queueLengthThreshold`: Scale-up when queue ≥ threshold (default: 5)
  - `kvSpareTrigger`: Scale-up when spare capacity < trigger (default: 0.10)
  - `queueSpareTrigger`: Scale-up when spare queue < trigger (default: 3)

### 2. Model Engine (Future)

**Location**: `internal/engines/model/`

**Purpose**: Predictive scaling using queueing theory models.

**Status**: Placeholder implementation for future use.

**Planned Features**:
- Queue-theoretic performance modeling (M/M/1/k, M/G/1)
- Predictive scaling based on anticipated load
- Cost-optimized allocation across variants
- Global optimization considering all variants

**Components**:
- **Model Analyzer** (`internal/modelanalyzer/`): Performance modeling
- **Optimizer** (`internal/optimizer/`, `pkg/solver/`): Global optimization
- **Queue Analyzer** (`pkg/analyzer/`): Queueing theory models

### 3. Scale-from-Zero Engine (Future)

**Location**: `internal/engines/scalefromzero/`

**Purpose**: Fast scale-up from zero replicas on demand.

**Status**: Placeholder implementation for future use.

**Planned Features**:
- Rapid detection of incoming requests
- Fast scale-up to minimum replicas
- Integration with gateway/router metrics
- Hybrid polling + event-driven execution

## Common Engine Infrastructure

### Decision Cache (`internal/engines/common/`)

**Purpose**: Thread-safe in-memory cache for passing scaling decisions from engines to controller.

**Key Features**:
- Zero API server overhead
- Per-variant decision storage
- Atomic read/write operations
- Event-driven reconciliation triggers

**Structure**:
```go
type VariantDecision struct {
    TargetReplicas   int
    CurrentReplicas  int
    AcceleratorName  string
    Cost             float64
    ScaleReason      string
    LastUpdated      time.Time
}
```

### Executor Strategies

**Polling Executor** (Current):
- Fixed-interval execution
- Configurable interval (30s default)
- Retry with backoff on failures
- Use case: Saturation engine, model engine

**Reactive Executor** (Future):
- Event-driven execution
- Triggered by metrics changes
- Lower latency response
- Use case: Scale-from-zero

**Hybrid Executor** (Future):
- Combines polling and reactive
- Periodic baseline + event response
- Optimal for mixed workloads

## Interfaces and Contracts

### Core Interfaces (`internal/interfaces/`)

**MetricsCollector**:
```go
type MetricsCollector interface {
    CollectSaturationMetrics(ctx, va) ([]ReplicaMetrics, error)
    CollectModelMetrics(ctx, va) (*ModelMetrics, error)
}
```

**SaturationAnalyzer**:
```go
type SaturationAnalyzer interface {
    AnalyzeModelSaturation(ctx, modelID, namespace, 
        replicaMetrics, config) (*ModelSaturationAnalysis, error)
}
```

**Engine**:
```go
type Engine interface {
    StartOptimizeLoop(ctx context.Context)
    StopOptimizeLoop()
}
```

## Integration with External Autoscalers

WVA emits recommendations via Prometheus metrics that HPA/KEDA consumes:

```
┌─────────────┐
│ WVA Actuator│
└──────┬──────┘
       │ emits: inferno_desired_replicas
       │        inferno_current_replicas
       ▼
┌─────────────┐
│ Prometheus  │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  HPA/KEDA   │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Deployment  │
└─────────────┘
```

**Metrics**:
- `inferno_desired_replicas`: Target replica count from WVA
- `inferno_current_replicas`: Current observed replicas
- `inferno_optimization_timestamp`: Last optimization time

## Configuration and Tuning

### Saturation Engine Configuration

**ConfigMap**: `capacity-scaling-config`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: capacity-scaling-config
  namespace: workload-variant-autoscaler-system
data:
  default: |
    kvCacheThreshold: 0.80
    queueLengthThreshold: 5
    kvSpareTrigger: 0.10
    queueSpareTrigger: 3
  meta-llama-3.1-8b: |
    kvCacheThreshold: 0.75
    queueLengthThreshold: 10
```

### Collector Configuration

**Caching** (`internal/collector/config/`):
- Cache TTL: 30s (default)
- Background refresh: Enabled
- Max cache size: Unlimited (in-memory)

**Prometheus Connection**:
- TLS support with CA certificate
- Bearer token authentication
- Connection timeout: 30s

### Engine Selection

**Environment Variable**: `ENGINE_MODE`
- `saturation`: Saturation-based scaling (default)
- `model`: Model-based predictive scaling (future)
- `scalefromzero`: Scale-from-zero engine (future)

## Performance Characteristics

### Saturation Engine

**Latency**:
- Metric collection: ~100-500ms (Prometheus query)
- Analysis: <10ms (per-variant calculation)
- Total cycle: ~1-2s

**Scalability**:
- Supports 100+ variants per cluster
- Collector cache reduces Prometheus load
- Background fetching for proactive updates

**Resource Usage**:
- Memory: ~50MB base + ~1MB per 100 variants
- CPU: <0.1 core average, <0.5 core during optimization

## Monitoring and Observability

### Controller Metrics

**Exposed by WVA**:
- `wva_reconcile_duration_seconds`: Reconciliation latency
- `wva_reconcile_errors_total`: Reconciliation errors
- `wva_optimization_duration_seconds`: Optimization cycle duration

### Engine Metrics

**Saturation Engine**:
- `wva_saturation_analysis_duration_seconds`: Analysis time
- `wva_collector_cache_hits_total`: Cache efficiency
- `wva_collector_prometheus_queries_total`: Prometheus API load

### Logs

**Structured logging** with log levels:
- `INFO`: Normal operations, scaling decisions
- `DEBUG`: Detailed metrics, cache operations
- `ERROR`: Failures, invalid configurations

**Key log messages**:
```
"Saturation analysis complete" variant=llama-8b desired=5 current=3 reason="spare_capacity_low"
"Scale-down blocked" variant=llama-8b reason="safety_simulation_failed"
"Collector cache miss" key=llama-8b-metrics age=45s
```

## Testing

### Unit Tests

**Saturation Engine**:
- `internal/engines/saturation/engine_test.go`
- `internal/saturation/analyzer_test.go`
- `internal/collector/prometheus/saturation_metrics_test.go`

### E2E Tests

**Saturation-based E2E**:
- Location: `test/e2e-saturation-based/`
- Tests: Scale-up on saturation, multi-variant scaling, cost optimization
- Environment: Kind cluster with GPU emulation

**OpenShift E2E**:
- Location: `test/e2e-openshift/`
- Tests: Real GPU workloads, production scenarios

## Troubleshooting

### Common Issues

**Metrics not collected**:
- Check Prometheus connectivity: `kubectl logs -n workload-variant-autoscaler-system deployment/wva-controller-manager`
- Verify vLLM metrics exposed: `kubectl port-forward svc/prometheus 9090:9090`
- Query: `vllm:kv_cache_usage_perc`

**No scaling decisions**:
- Check ConfigMap exists: `kubectl get cm capacity-scaling-config -n workload-variant-autoscaler-system`
- Verify VA status: `kubectl get va -n <namespace> <name> -o yaml`
- Check decision cache: Enable DEBUG logging

**Unexpected scale-down**:
- Review spare capacity thresholds in ConfigMap
- Check safety simulation logs
- Verify replica readiness (only ready replicas counted)

## Migration Guide

### From v0.3 to v0.4 (Saturation Engine)

**Breaking Changes**:
1. Engine mode now required (defaults to `saturation`)
2. ConfigMap `capacity-scaling-config` must be deployed
3. Prometheus metrics namespace changed to `vllm:`

**Migration Steps**:
1. Deploy updated CRDs: `kubectl apply -f charts/workload-variant-autoscaler/crds/`
2. Deploy ConfigMap: `kubectl apply -f deploy/configmap-capacity-scaling.yaml`
3. Upgrade WVA: `helm upgrade workload-variant-autoscaler ./charts/workload-variant-autoscaler`
4. Verify metrics: Check Prometheus for `inferno_desired_replicas`

## Future Roadmap

### Model Engine (v0.5)

- [ ] Queueing theory integration
- [ ] Predictive scaling based on load forecasts
- [ ] Global optimization across variants
- [ ] Hybrid mode (saturation + model)

### Scale-from-Zero Engine (v0.6)

- [ ] Gateway metrics integration
- [ ] Fast scale-up (<10s from 0 to 1 replica)
- [ ] Reactive executor implementation
- [ ] Idle detection and scale-down

### Enhancements

- [ ] Multi-cluster support
- [ ] Custom metrics adapters
- [ ] Advanced cost models (spot instances, reserved capacity)
- [ ] ML-based anomaly detection

## References

- [Saturation Analyzer Documentation](../saturation-analyzer.md)
- [Saturation Scaling Configuration](../saturation-scaling-config.md)
- [Collector Implementation](../../internal/collector/README.md) (TODO)
- [E2E Saturation Tests](../../test/e2e-saturation-based/README.md)

## Contributing

Contributions to engine implementations welcome! See [Developer Guide](../developer-guide/development.md) and [Contributing Guidelines](../../CONTRIBUTING.md).

**Adding a new engine**:
1. Create package under `internal/engines/<engine-name>/`
2. Implement `Engine` interface
3. Add executor strategy
4. Create unit and E2E tests
5. Update documentation
