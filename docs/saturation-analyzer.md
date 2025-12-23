# Saturation Analyzer

## Overview

The Saturation Analyzer is a **fast, reactive, and safe saturation guardrail** that prevents capacity exhaustion by monitoring live vLLM metrics. It analyzes KV cache utilization and queue depth across all replicas to make intelligent scaling decisions.

**Key Features:**
- ✅ Operates from live vLLM metrics (no offline profiling required)
- ✅ Detects imminent capacity exhaustion (KV-cache or request queue)
- ✅ Analyzes saturation across all variants of the same model
- ✅ Performs worst-case scale-down safety simulation
- ✅ Supports per-model configuration overrides via ConfigMap
- ✅ Thread-safe with efficient caching and automatic config reload

## Architecture

### Components

**1. Saturation Analyzer (`internal/saturation/analyzer.go`)**
- Core analysis logic for saturation-based scaling decisions
- Implements spare capacity calculations using average spare capacity model
- Performs worst-case scale-down safety simulation (total load redistribution)
- Analyzes saturation per-variant with aggregated model-level decisions
- Returns structured analysis results with per-variant breakdown

**2. Saturation Engine (`internal/engines/saturation/engine.go`)**
- Orchestrates the saturation-based autoscaling workflow
- Integrates with metrics collector, analyzer, and actuator
- Runs periodic optimization loop using polling executor
- Manages ConfigMap-based configuration with automatic reload
- Coordinates with external autoscalers (HPA/KEDA) via metrics

**3. Metrics Collector (`internal/collector/prometheus/saturation_metrics.go`)**
- Collects vLLM metrics from Prometheus using `max_over_time[1m]` queries
- Queries `constants.VLLMKvCacheUsagePerc` and `constants.VLLMNumRequestsWaiting`
- Uses peak values over 1 minute for safety-first saturation analysis
- Enriches metrics with pod metadata (variant name, accelerator type, model ID)

**4. Interfaces (`internal/interfaces/saturation_analyzer.go`, `internal/interfaces/saturation_scaling.go`)**
- Defines data structures for replica metrics and saturation analysis results
- Defines `SaturationAnalyzer` interface for pluggable analysis implementations
- Defines `SaturationScalingConfig` for threshold configuration
- Provides `ModelSaturationAnalysis` and `VariantSaturationAnalysis` result types

### Data Flow

```
┌─────────────┐
│  Prometheus │
└──────┬──────┘
       │ vLLM metrics (KV cache, queue length)
       ↓
┌──────────────────────────┐
│ MetricsCollector         │
│ (Prometheus)             │
└────────┬─────────────────┘
         │ ReplicaMetrics[]
         ↓
┌──────────────────────────┐
│ SaturationAnalyzer       │  ← SaturationScalingConfig
│ (internal/saturation)    │     (from ConfigMap)
└────────┬─────────────────┘
         │ ModelSaturationAnalysis
         │ (with per-variant breakdown)
         ↓
┌──────────────────────────┐
│ SaturationEngine         │
│ - Determines replicas    │
│ - Updates CRD status     │
└────────┬─────────────────┘
         │ Emits metrics
         ↓
┌──────────────────────────┐
│ Actuator                 │
│ - Pushes to Prometheus   │
└────────┬─────────────────┘
         │ External metrics
         ↓
┌──────────────────────────┐
│ External Autoscaler      │
│ (HPA or KEDA)            │
└──────────────────────────┘
```

## Analysis Algorithm

### Step 1: Identify Non-Saturated Replicas

A replica is **non-saturated** if:
```
kv_cache_usage < kvCacheThreshold AND queue_length < queueLengthThreshold
```

**Default thresholds:**
- `kvCacheThreshold`: 0.80 (80%)
- `queueLengthThreshold`: 5

### Step 2: Calculate Spare Capacity

For each non-saturated replica:
```
spare_kv_i = kvCacheThreshold - kv_cache_usage_i
spare_queue_i = queueLengthThreshold - queue_length_i
```

### Step 3: Average Spare Capacity

Across all non-saturated replicas:
```
avg_spare_kv = Σ spare_kv_i / N_non_sat
avg_spare_queue = Σ spare_queue_i / N_non_sat
```

### Step 4: Scale-Up Decision

Trigger scale-up if:
```
avg_spare_kv < kvSpareTrigger OR avg_spare_queue < queueSpareTrigger
```

**Default triggers:**
- `kvSpareTrigger`: 0.1 (10%)
- `queueSpareTrigger`: 3

This **proactive approach** scales up before replicas become fully saturated, ensuring adequate headroom.

### Step 5: Scale-Down Safety Simulation

Before allowing scale-down, the analyzer simulates total load redistribution across remaining replicas:

```
remaining_replicas = N_non_sat - 1

// Calculate total load across all non-saturated replicas
total_kv_load = Σ kv_cache_usage_i (for all non-saturated replicas)
total_queue_load = Σ queue_length_i (for all non-saturated replicas)

// Simulate removing one replica: redistribute total load
avg_kv_after_removal = total_kv_load / remaining_replicas
avg_queue_after_removal = total_queue_load / remaining_replicas

// Calculate remaining spare capacity
remaining_spare_kv = kvCacheThreshold - avg_kv_after_removal
remaining_spare_queue = queueLengthThreshold - avg_queue_after_removal
```

**Scale-down is safe if:**
```
remaining_spare_kv >= kvSpareTrigger AND
remaining_spare_queue >= queueSpareTrigger AND
N_non_sat >= 2
```

This worst-case simulation prevents premature scale-down that could lead to saturation.

## Usage Examples

### Basic Analysis Flow

```go
import (
    "context"
    "github.com/llm-d-incubation/workload-variant-autoscaler/internal/saturation"
    "github.com/llm-d-incubation/workload-variant-autoscaler/internal/collector/prometheus"
    "github.com/llm-d-incubation/workload-variant-autoscaler/internal/interfaces"
)

// Create analyzer
analyzer := saturation.NewAnalyzer()

// Collect metrics (uses max_over_time[1m] for safety-first analysis)
metricsCollector := prometheus.NewCollector(promAPI, k8sClient)
replicaMetrics, err := metricsCollector.CollectSaturationMetrics(ctx, modelID, namespace)

// Get saturation config (from ConfigMap cache)
config := interfaces.DefaultSaturationScalingConfig()

// Analyze saturation across all variants
analysis, err := analyzer.AnalyzeModelSaturation(ctx, modelID, namespace, replicaMetrics, config)

// Check analysis results
log.Printf("Total replicas: %d", analysis.TotalReplicas)
log.Printf("Non-saturated: %d", analysis.NonSaturatedCount)
log.Printf("Should scale up: %v", analysis.ShouldScaleUp)
log.Printf("Scale-down safe: %v", analysis.ScaleDownSafe)

// Per-variant breakdown
for _, va := range analysis.VariantAnalyses {
    log.Printf("Variant: %s", va.VariantName)
    log.Printf("  Replicas: %d", va.ReplicaCount)
    log.Printf("  Non-saturated: %d", va.NonSaturatedCount)
    log.Printf("  Avg spare KV: %.3f", va.AvgSpareKvCapacity)
    log.Printf("  Avg spare queue: %.1f", va.AvgSpareQueueLength)
}

// Make scaling decision
if analysis.ShouldScaleUp {
    log.Printf("Scaling up: spare capacity low")
    // Trigger scale-up
} else if analysis.ScaleDownSafe {
    log.Printf("Scale-down safe: sufficient spare capacity")
    // Allow scale-down
} else {
    log.Printf("No change: current capacity adequate")
}
```

## Multi-Variant Analysis

The saturation analyzer aggregates metrics **across all variants of the same model**:

```go
// Example: Model "llama-70b" with 2 variants
// - variant-1 (A100, 2 replicas)
// - variant-2 (H100, 3 replicas)

replicaMetrics := []interfaces.ReplicaMetrics{
    // Variant 1
    {PodName: "v1-pod-1", VariantName: "variant-1", ModelID: "llama-70b",
     AcceleratorName: "A100", KvCacheUsage: 0.70, QueueLength: 2},
    {PodName: "v1-pod-2", VariantName: "variant-1", ModelID: "llama-70b",
     AcceleratorName: "A100", KvCacheUsage: 0.75, QueueLength: 3},

    // Variant 2
    {PodName: "v2-pod-1", VariantName: "variant-2", ModelID: "llama-70b",
     AcceleratorName: "H100", KvCacheUsage: 0.60, QueueLength: 1},
    {PodName: "v2-pod-2", VariantName: "variant-2", ModelID: "llama-70b",
     AcceleratorName: "H100", KvCacheUsage: 0.65, QueueLength: 2},
    {PodName: "v2-pod-3", VariantName: "variant-2", ModelID: "llama-70b",
     AcceleratorName: "H100", KvCacheUsage: 0.55, QueueLength: 1},
}

// Analyzer aggregates across all 5 replicas
analysis, _ := analyzer.AnalyzeModelSaturation(ctx, "llama-70b", "prod", replicaMetrics, config)

// Results include per-variant breakdown
fmt.Printf("Total replicas: %d\n", analysis.TotalReplicas) // 5
fmt.Printf("Non-saturated: %d\n", analysis.NonSaturatedCount) // 5
fmt.Printf("Variants analyzed: %d\n", len(analysis.VariantAnalyses)) // 2

for _, va := range analysis.VariantAnalyses {
    fmt.Printf("Variant: %s, Replicas: %d, Accelerator: %s\n",
        va.VariantName, va.ReplicaCount, va.AcceleratorName)
}

// Model-level decision (aggregated)
if analysis.ShouldScaleUp {
    fmt.Println("Scale up needed (any variant)")
}
if analysis.ScaleDownSafe {
    fmt.Println("Scale down safe (all variants)")
}
```

## Configuration

Saturation scaling thresholds are configured via ConfigMap (see [saturation-scaling-config.md](saturation-scaling-config.md)):

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: saturation-scaling-config
  namespace: workload-variant-autoscaler-system
data:
  default: |
    kvCacheThreshold: 0.80
    queueLengthThreshold: 5
    kvSpareTrigger: 0.1
    queueSpareTrigger: 3
```

**Per-model overrides:**
```yaml
  llama-70b-prod: |
    model_id: meta/llama-70b
    namespace: production
    kvCacheThreshold: 0.85
    kvSpareTrigger: 0.15
```

Configuration is:
- ✅ Cached on startup (single read)
- ✅ Automatically reloaded on ConfigMap changes
- ✅ Thread-safe for concurrent access
- ✅ Falls back to defaults if ConfigMap missing

## Testing

Comprehensive unit tests are provided in `internal/saturation/analyzer_test.go`:

```bash
cd internal/saturation
go test -v
```

**Test coverage:**
- ✅ Scale-up trigger conditions
- ✅ Scale-down safety simulation (total load redistribution)
- ✅ Multi-variant aggregation
- ✅ Saturated replica identification
- ✅ Edge cases (empty metrics, single replica, nil analysis)
- ✅ Per-variant analysis correctness
- ✅ Configuration validation

## Observability

### Log Messages

**Saturation analysis:**
```
DEBUG Saturation analysis completed
  modelID=llama-70b
  namespace=prod
  totalReplicas=5
  nonSaturated=4
  avgSpareKv=0.150
  avgSpareQueue=2.5
  shouldScaleUp=true
  scaleDownSafe=false
```

**Scale-down safety:**
```
DEBUG Scale-down unsafe: insufficient headroom after redistribution
  remainingSpareKv=0.050
  kvTrigger=0.100
  kvSafe=false
  remainingSpareQueue=1.0
  queueTrigger=3
  queueSafe=false
```

**Per-variant analysis:**
```
DEBUG Variant saturation analysis
  variant=variant-1
  modelID=llama-70b
  replicas=2
  nonSaturated=2
  avgSpareKv=0.180
  avgSpareQueue=3.5
```

## Performance Characteristics

### Computational Complexity

- **Per-replica analysis:** O(N) where N = number of replicas
- **Variant aggregation:** O(V) where V = number of variants
- **Overall:** O(N + V), typically O(N) as V << N

### Prometheus Queries

**Two queries per model:**
1. `max_over_time(vllm:kv_cache_usage_perc{namespace="prod",model_id="llama-70b"}[1m])` (returns N samples with peak values)
2. `max_over_time(vllm:num_requests_waiting{namespace="prod",model_id="llama-70b"}[1m])` (returns N samples with peak values)

**Query strategy:** Uses `max_over_time[1m]` to capture peak saturation usage in the last minute, providing conservative safety-first analysis that prevents missing saturation events between queries. The `model_id` filter ensures metrics are scoped to the specific model being analyzed, preventing cross-model metric pollution.

**Query frequency:** Once per reconciliation loop (configurable, typically every 30s)

## Integration Notes

### Saturation Engine Integration

The saturation analyzer is integrated into the saturation engine's optimization loop:

1. **Collect metrics** for all pods of a model (across all variants)
   - Prometheus collector gathers KV cache and queue metrics
   - Metrics enriched with pod metadata (variant, model, accelerator)

2. **Analyze saturation** using `AnalyzeModelSaturation`
   - Aggregates metrics across all variants
   - Produces `ModelSaturationAnalysis` with per-variant breakdown

3. **Determine replica count** based on analysis
   - Scale up if `ShouldScaleUp == true`
   - Scale down if `ScaleDownSafe == true`
   - Maintain current replicas otherwise

4. **Update CRD status** with current/desired replicas
   - Status reflects saturation-based decisions
   - Status used by external autoscalers

5. **Emit metrics** via Actuator
   - Push to Prometheus for external consumption
   - External autoscaler (HPA/KEDA) reads metrics and scales deployment

### Metrics Requirements

The analyzer requires these Prometheus metrics from vLLM (defined in `internal/constants/metrics.go`):
- `vllm:kv_cache_usage_perc` — KV cache utilization (0.0-1.0)
- `vllm:num_requests_waiting` — Queue length (integer)

These metrics must include the following labels:
- `pod` or `pod_name` — Pod identification
- `model_id` — Model identification (to prevent cross-model metric pollution)
- `namespace` — Kubernetes namespace

## Limitations

1. **Minimum replicas:** Scale-down requires ≥2 non-saturated replicas for safety simulation
2. **Metric availability:** Assumes vLLM metrics are available in Prometheus
3. **Pod identification:** Requires pod and model_id labels in Prometheus metrics
4. **No model profiling:** Does not account for model-specific saturation curves
5. **Threshold-based:** Uses fixed thresholds (configurable per-model via ConfigMap)

## Future Enhancements

Potential improvements:
- Per-accelerator type threshold overrides
- Historical saturation trend analysis
- Predictive saturation planning
- Integration with Inference Scheduler thresholds
- Metric-based cache invalidation
- ML-based saturation prediction

## References
- Related: [Saturation Scaling Configuration](saturation-scaling-config.md)
- Engine: [Saturation Engine](../internal/engines/saturation/)
- Tests: [Analyzer Tests](../internal/saturation/analyzer_test.go)

