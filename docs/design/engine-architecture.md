# Engine Architecture

## Overview

The Workload Variant Autoscaler (WVA) uses an **engine-based architecture** that separates optimization logic from execution strategy. This design enables different autoscaling approaches (saturation-based, model-based, scale-from-zero) to coexist and evolve independently.

## Architecture Principles

1. **Separation of Concerns**: Optimization logic (engine) is decoupled from execution strategy (executor)
2. **Pluggable Engines**: Multiple engine implementations for different autoscaling approaches
3. **Flexible Execution**: Support for polling, reactive, and hybrid execution strategies
4. **Configuration-Driven**: Behavior controlled via ConfigMaps and environment variables
5. **Observable**: Comprehensive logging and metrics for troubleshooting

## Components

### 1. Engines (`internal/engines/`)

Engines implement autoscaling logic for different approaches:

#### Saturation Engine (`internal/engines/saturation/`)

**Purpose:** Reactive autoscaling based on live vLLM saturation metrics (KV cache, queue depth)

**Key Features:**
- Monitors KV cache utilization and queue length across all replicas
- Performs spare capacity analysis with proactive scale-up triggers
- Implements worst-case scale-down safety simulation
- Supports per-model configuration via ConfigMap
- Emits metrics for external autoscalers (HPA/KEDA)

**Configuration:**
```go
// Environment variables
POD_NAMESPACE                    // Namespace for ConfigMap lookup
CONFIG_MAP_NAME                  // Main config (default: workload-variant-autoscaler-variantautoscaling-config)
SATURATION_CONFIG_MAP_NAME       // Saturation thresholds (default: saturation-scaling-config)
WVA_SCALE_TO_ZERO               // Enable scale-to-zero (default: false)
```

**Workflow:**
1. Fetch active VariantAutoscaling resources
2. Collect vLLM metrics from Prometheus (KV cache, queue depth)
3. Analyze saturation using `internal/saturation.Analyzer`
4. Determine target replicas based on analysis
5. Update VariantAutoscaling status with current/desired replicas
6. Emit metrics to Prometheus via Actuator
7. External autoscaler (HPA/KEDA) reads metrics and scales deployment

**Files:**
- `engine.go`: Main engine implementation with optimization loop
- `engine_test.go`: Comprehensive unit tests
- `suite_test.go`: Test suite setup

#### Model Engine (`internal/engines/model/`)

**Purpose:** Model-based autoscaling using queueing theory and performance models

**Status:** Planned (currently uses saturation engine by default)

**Key Features (Planned):**
- Queueing theory-based performance modeling (M/M/1/k, M/G/1)
- ITL (inter-token latency) prediction based on batch size
- SLO-aware optimization (TTFT, ITL targets)
- Cost-optimized allocation across variants

#### Scale-From-Zero Engine (`internal/engines/scalefromzero/`)

**Purpose:** Handle cold-start scenarios and scale-to-zero use cases

**Status:** Stub implementation

**Key Features (Planned):**
- Detect zero-replica scenarios
- Warm-up management
- Predictive scaling based on traffic patterns

### 2. Executors (`internal/engines/executor/`)

Executors define **how** and **when** optimization runs:

#### Polling Executor (`executor/polling.go`)

**Purpose:** Fixed-interval execution (current default)

**Configuration:**
```go
PollingConfig{
    Interval:     30 * time.Second,  // How often to run optimization
    RetryBackoff: 100 * time.Millisecond,  // Backoff on errors
    Config: executor.Config{
        OptimizeFunc: engine.optimize,  // Optimization function to call
    },
}
```

**Behavior:**
- Runs optimization every `Interval` (e.g., 30 seconds)
- Retries on errors with exponential backoff
- Continues until context cancelled
- Thread-safe

**Use Cases:**
- Steady-state autoscaling
- Predictable optimization frequency
- Low-latency environments with frequent metric updates

#### Reactive Executor (Planned)

**Purpose:** Event-driven execution

**Triggers (Planned):**
- ConfigMap changes
- VariantAutoscaling resource updates
- Metric threshold crossings
- External signals (webhooks, pub/sub)

**Benefits:**
- Lower latency response to changes
- Reduced unnecessary optimization cycles
- Better resource utilization

#### Hybrid Executor (Planned)

**Purpose:** Combined polling + reactive

**Benefits:**
- Baseline polling for safety
- Reactive for urgent events
- Best of both worlds

### 3. Supporting Components

#### Collector (`internal/collector/`)

**Purpose:** Gather cluster state and metrics

**Implementations:**
- **Prometheus Collector** (`prometheus/`): Collect vLLM metrics from Prometheus
  - `saturation_metrics.go`: KV cache and queue depth metrics
  - `background_fetching.go`: Async metric collection with caching
  - `cache_operations.go`: Thread-safe metric cache

**Key Features:**
- Uses `max_over_time[1m]` for safety-first peak value collection
- Enriches metrics with pod metadata (variant, model, accelerator)
- Background fetching with configurable refresh intervals
- Thread-safe caching with automatic expiration

#### Analyzer (`internal/saturation/`)

**Purpose:** Perform saturation analysis on collected metrics

**Key Components:**
- `analyzer.go`: Spare capacity analysis, scale-up/down decision logic
- `shared.go`: Shared utilities and helper functions
- `constants.go`: Default thresholds and configuration

**Algorithm:**
1. Identify non-saturated replicas
2. Calculate spare capacity (KV and queue)
3. Average spare capacity across non-saturated replicas
4. Trigger scale-up if spare capacity < trigger threshold
5. Simulate scale-down and verify safety

#### Actuator (`internal/actuator/`)

**Purpose:** Execute scaling decisions and emit metrics

**Responsibilities:**
- Update VariantAutoscaling CRD status
- Push metrics to Prometheus
- Record Kubernetes events
- Coordinate with external autoscalers

#### Interfaces (`internal/interfaces/`)

**Purpose:** Define contracts between components

**Key Interfaces:**
- `MetricsCollector`: Metric collection abstraction
- `SaturationAnalyzer`: Saturation analysis abstraction
- `SaturationScalingConfig`: Configuration structure
- Various result types: `ModelSaturationAnalysis`, `ReplicaMetrics`, etc.

## Configuration Management

### ConfigMaps

**1. Main Config (`workload-variant-autoscaler-variantautoscaling-config`)**
- Optimization interval
- Feature flags
- Global settings

**2. Saturation Config (`saturation-scaling-config`)**
- KV cache thresholds
- Queue length thresholds
- Spare capacity triggers
- Per-model overrides

**Loading Strategy:**
- Single read on startup (cached in memory)
- Automatic reload on ConfigMap changes via Kubernetes watch
- Thread-safe concurrent access with RWMutex
- Graceful degradation to defaults if ConfigMap missing

### Environment Variables

```bash
# Namespace and ConfigMap names
POD_NAMESPACE=workload-variant-autoscaler-system
CONFIG_MAP_NAME=workload-variant-autoscaler-variantautoscaling-config
SATURATION_CONFIG_MAP_NAME=saturation-scaling-config

# Feature flags
WVA_SCALE_TO_ZERO=false

# Prometheus configuration
PROMETHEUS_URL=http://prometheus-server.monitoring:9090
PROMETHEUS_CA_CERT=/path/to/ca.crt
PROMETHEUS_CLIENT_CERT=/path/to/client.crt
PROMETHEUS_CLIENT_KEY=/path/to/client.key
```

## Execution Flow

### Saturation Engine Flow

```
┌──────────────────────────────────────────┐
│  PollingExecutor.Start(ctx)              │
│  (every 30s)                             │
└────────────┬─────────────────────────────┘
             ↓
┌──────────────────────────────────────────┐
│  SaturationEngine.optimize(ctx)          │
│  1. Read optimization config             │
│  2. Get active VariantAutoscaling CRs    │
└────────────┬─────────────────────────────┘
             ↓
┌──────────────────────────────────────────┐
│  For each VariantAutoscaling:            │
│  ┌────────────────────────────────────┐  │
│  │ 1. Collect metrics (Prometheus)    │  │
│  │ 2. Analyze saturation              │  │
│  │ 3. Determine target replicas       │  │
│  │ 4. Update CRD status               │  │
│  │ 5. Emit metrics to Prometheus      │  │
│  └────────────────────────────────────┘  │
└────────────┬─────────────────────────────┘
             ↓
┌──────────────────────────────────────────┐
│  External Autoscaler (HPA/KEDA)          │
│  - Reads metrics from Prometheus         │
│  - Scales deployment to desired replicas │
└──────────────────────────────────────────┘
```

### Detailed Saturation Analysis Flow

```
1. Metrics Collection (Prometheus Collector)
   ↓
   max_over_time(vllm:kv_cache_usage_perc[1m])
   max_over_time(vllm:num_requests_waiting[1m])
   ↓
2. Saturation Analysis (internal/saturation.Analyzer)
   ↓
   • Identify non-saturated replicas (below thresholds)
   • Calculate spare capacity per replica
   • Average spare capacity across non-saturated replicas
   • Compare to triggers (kvSpareTrigger, queueSpareTrigger)
   • Simulate scale-down (worst-case load redistribution)
   ↓
3. Decision Logic (SaturationEngine)
   ↓
   if ShouldScaleUp:
       desired_replicas = current + 1
   elif ScaleDownSafe:
       desired_replicas = current - 1
   else:
       desired_replicas = current
   ↓
4. Status Update
   ↓
   Update VariantAutoscaling.Status.CurrentReplicas
   Update VariantAutoscaling.Status.DesiredReplicas
   ↓
5. Metrics Emission (Actuator)
   ↓
   Push to Prometheus:
   - wva_desired_replicas{model_id, namespace}
   - wva_current_replicas{model_id, namespace}
   - wva_saturation_kv_spare_capacity{model_id, namespace}
   - wva_saturation_queue_spare_length{model_id, namespace}
```

## Testing Strategy

### Unit Tests

**Analyzer Tests** (`internal/saturation/analyzer_test.go`):
- Scale-up trigger conditions
- Scale-down safety simulation
- Multi-variant aggregation
- Edge cases (empty metrics, single replica)

**Engine Tests** (`internal/engines/saturation/engine_test.go`):
- Optimization loop correctness
- ConfigMap integration
- Error handling
- Status update logic

**Executor Tests** (`internal/engines/executor/*_test.go`):
- Polling interval adherence
- Error retry logic
- Context cancellation
- Thread safety

### Integration Tests

**E2E Saturation Tests** (`test/e2e-saturation-based/`):
- End-to-end saturation-based autoscaling
- Metric collection → analysis → scaling
- External autoscaler integration

**E2E OpenShift Tests** (`test/e2e-openshift/`):
- OpenShift-specific scenarios
- Production-like workloads

## Observability

### Logging

**Log Levels:**
- `DEBUG`: Detailed analysis results, metric values
- `INFO`: Scaling decisions, optimization cycles
- `WARN`: Configuration issues, metric collection failures
- `ERROR`: Unrecoverable errors

**Key Log Messages:**
```
INFO  Starting saturation engine optimization loop
DEBUG Saturation analysis completed modelID=llama-70b totalReplicas=5 shouldScaleUp=true
INFO  Updated VariantAutoscaling status modelID=llama-70b current=5 desired=6
WARN  Failed to collect metrics modelID=llama-70b error="prometheus unreachable"
ERROR Unable to update VariantAutoscaling status error="resource not found"
```

### Metrics

**Emitted to Prometheus:**
- `wva_desired_replicas`: Target replicas determined by WVA
- `wva_current_replicas`: Current replicas
- `wva_saturation_kv_spare_capacity`: Spare KV cache capacity
- `wva_saturation_queue_spare_length`: Spare queue length
- `wva_optimization_duration_seconds`: Time to complete optimization
- `wva_optimization_errors_total`: Count of optimization errors

**Consumed from Prometheus:**
- `vllm:kv_cache_usage_perc`: KV cache utilization per pod
- `vllm:num_requests_waiting`: Queue depth per pod

## Migration and Evolution

### Current State (v0.5.0)

- ✅ Saturation engine (production-ready)
- ✅ Polling executor (default)
- ⏳ Model engine (planned)
- ⏳ Reactive executor (planned)
- ⏳ Scale-from-zero engine (stub)

### Migration Path

**From older controller-based architecture:**
1. Engine pattern introduced in PR #460
2. Existing reconciliation logic moved to saturation engine
3. Executor pattern separates "what" (optimization) from "when" (execution)
4. No breaking changes to CRD or external APIs

**Future enhancements:**
1. Reactive executor: Event-driven optimization
2. Model engine: Queueing theory-based optimization
3. Hybrid executor: Combined polling + reactive
4. Multi-engine coordination: Saturation + model-based hybrid decisions

## Best Practices

### For Operators

1. **Start with defaults**: Use default saturation thresholds initially
2. **Monitor metrics**: Watch spare capacity metrics to tune thresholds
3. **Per-model tuning**: Override thresholds per model based on SLO requirements
4. **Coordinate with EPP**: Align WVA thresholds with Inference Scheduler thresholds
5. **External autoscaler**: Configure HPA stabilization window (120s+)

### For Developers

1. **Follow interfaces**: Implement defined interfaces for pluggability
2. **Test thoroughly**: Unit tests + integration tests for new engines
3. **Log decisions**: Always log scaling decisions with context
4. **Configuration-driven**: Avoid hardcoded behavior, use ConfigMaps
5. **Thread-safe**: All shared state must be protected (RWMutex)

## Troubleshooting

### Engine Not Running

**Symptom:** No optimization cycles in logs

**Check:**
```bash
kubectl logs -n workload-variant-autoscaler-system deployment/wva-controller | grep "optimization loop"
```

**Causes:**
- Controller pod not running
- Context cancelled prematurely
- Panic in optimize function

### ConfigMap Not Loaded

**Symptom:** Warning about missing ConfigMap

**Check:**
```bash
kubectl get cm saturation-scaling-config -n workload-variant-autoscaler-system
```

**Solution:**
```bash
kubectl apply -f deploy/configmap-saturation-scaling.yaml
```

### Metrics Not Collected

**Symptom:** "Failed to collect metrics" errors

**Check:**
1. Prometheus connectivity: `kubectl get svc -n monitoring`
2. vLLM metrics exist: Query Prometheus for `vllm:kv_cache_usage_perc`
3. TLS configuration: Check certificates if using mTLS

### No Scaling Actions

**Symptom:** Analysis shows scale-up needed but replicas unchanged

**Check:**
1. Status updated: `kubectl get va <name> -o yaml | grep status`
2. Metrics emitted: Query Prometheus for `wva_desired_replicas`
3. External autoscaler: `kubectl get hpa` or `kubectl get scaledobject`

## References

- [Saturation Analyzer Documentation](../saturation-analyzer.md)
- [Saturation Scaling Configuration](../saturation-scaling-config.md)
- [Executor Package Documentation](../../internal/engines/executor/doc.go)
- [Saturation Engine Implementation](../../internal/engines/saturation/engine.go)
- [Queue Analyzer (Model Engine)](../../pkg/analyzer/README.md)
