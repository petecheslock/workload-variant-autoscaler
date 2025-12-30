# Architecture Overview

This document provides a comprehensive overview of the Workload Variant Autoscaler (WVA) architecture, component interactions, and design principles.

## System Architecture

WVA follows a modular, pluggable architecture with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                       │
│                                                             │
│  ┌────────────────┐         ┌──────────────────┐          │
│  │ VariantAuto-   │◄────────┤  WVA Controller  │          │
│  │ scaling CRD    │         │                  │          │
│  └────────────────┘         └────────┬─────────┘          │
│                                      │                     │
│                            ┌─────────▼──────────┐         │
│                            │   Engine Layer     │         │
│                            │  (Saturation)      │         │
│                            └─────────┬──────────┘         │
│                                      │                     │
│        ┌─────────────┬───────────────┼───────────┬───────┐│
│        │             │               │           │       ││
│   ┌────▼────┐   ┌───▼────┐    ┌────▼────┐  ┌──▼──┐   ││
│   │Collector│   │Analyzer│    │Optimizer│  │Actor│   ││
│   └────┬────┘   └────────┘    └─────────┘  └──┬──┘   ││
│        │                                       │       ││
│   ┌────▼────────────────────┐         ┌──────▼──────┐ ││
│   │   Prometheus Server     │         │  Target     │ ││
│   │   (Metrics Backend)     │         │  Deployment │ ││
│   └─────────────────────────┘         └─────────────┘ ││
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Controller Layer (`internal/controller/`)

The **VariantAutoscaling Controller** is the main Kubernetes controller that:
- Watches `VariantAutoscaling` custom resources
- Coordinates the reconciliation loop
- Delegates autoscaling decisions to the engine layer
- Updates CR status with current and desired state

**Key Files:**
- `variantautoscaling_controller.go` - Main controller implementation
- `predicates.go` - Event filtering logic
- `allocation.go` - Allocation helper functions

### 2. Engine Layer (`internal/engines/`)

The engine layer provides pluggable autoscaling strategies. Currently, the **Saturation Engine** is the primary implementation:

#### Saturation Engine (`internal/engines/saturation/`)

Implements capacity-aware autoscaling based on real-time saturation metrics:
- Monitors vLLM KV-cache utilization and queue depth
- Makes scaling decisions based on spare capacity thresholds
- Supports cost-aware variant selection
- Prevents capacity exhaustion through proactive scaling

**Architecture:**
```
┌─────────────────────────────────────────────┐
│         Saturation Engine                   │
│                                             │
│  1. Collect Metrics ──────► ReplicaMetrics │
│  2. Analyze Saturation ───► Analysis       │
│  3. Calculate Targets ────► Targets        │
│  4. Optimize (optional) ──► Optimized      │
│  5. Actuate ──────────────► Update Status  │
└─────────────────────────────────────────────┘
```

**Related Components:**
- **Model Engine** (`internal/engines/model/`) - Queueing theory-based engine (alternative)
- **ScaleFromZero Engine** (`internal/engines/scalefromzero/`) - Cold-start handling
- **Executor** (`internal/engines/executor/`) - Task execution strategies (polling, reactive)
- **Common** (`internal/engines/common/`) - Shared utilities and caching

### 3. Collector (`internal/collector/`)

The collector subsystem gathers metrics from external sources:

**Interface:** `internal/interfaces/metrics_collector.go`

**Implementations:**
- **Prometheus Collector** (`internal/collector/prometheus/`) - Primary implementation
  - Queries vLLM metrics from Prometheus
  - Provides background metric fetching with caching
  - Implements query helpers and tracking

**Features:**
- Pluggable backend support (Prometheus, future: EPP)
- Metric caching to reduce query load
- Background fetching for improved responsiveness
- Query result tracking and validation

**Key Metrics Collected:**
- `vllm:gpu_cache_usage_perc` - KV-cache utilization
- `vllm:num_requests_waiting` - Queue depth
- Request rates and token counts
- Replica readiness and metadata

### 4. Saturation Analyzer (`internal/saturation/`)

Analyzes saturation metrics and determines scaling needs:

**Core Algorithm:**
1. **Aggregate Metrics** - Collect metrics from all replicas across variants
2. **Calculate Spare Capacity** - Determine available headroom
3. **Threshold Evaluation** - Compare against configured thresholds
4. **Safety Simulation** - Test scale-down impact through worst-case analysis
5. **Per-Variant Decisions** - Make cost-aware scaling recommendations

**Key Functions:**
- `AnalyzeModelSaturation()` - Analyzes all variants of a model
- `CalculateCapacityTargets()` - Determines target replica counts
- `ArbitrateWithModelBased()` - Hybrid decision with queueing model

**Configuration:** `interfaces.SaturationScalingConfig`
- `KvCacheThreshold` - KV-cache trigger point (default: 80%)
- `QueueDepthThreshold` - Queue depth trigger (default: 5)
- `SpareCapacityTarget` - Target spare capacity (default: 20%)
- `SafetyMargin` - Scale-down safety buffer (default: 10%)

### 5. Optimizer (`internal/optimizer/`)

Makes global optimization decisions across variants:

**Current Mode:** Unlimited
- Each variant receives independent optimal allocation
- No cluster capacity constraints
- Compatible with cluster autoscalers

**Future:** Limited mode with capacity-aware allocation

### 6. Actuator (`internal/actuator/`)

Executes scaling decisions:
- Emits custom metrics to Prometheus
- Updates deployment replicas (when direct actuation enabled)
- Records events to Kubernetes
- Updates CR status

**Metrics Emitted:**
- `wva_desired_replicas` - Target replica count
- `wva_current_replicas` - Observed replica count
- `wva_saturation_level` - Current saturation metrics
- `wva_scaling_decision` - Scale-up/down/stable indicator

### 7. Model Analyzer (`pkg/analyzer/`)

Provides queueing theory-based performance modeling:

**Models Supported:**
- **M/M/1/k** - Markovian queue with capacity limit
- **M/G/1** - General service time distribution

**Use Cases:**
- Performance prediction (latency, throughput)
- Capacity sizing (replicas needed for SLO)
- "What-if" analysis for deployment planning

**Key Metrics:**
- TTFT (Time To First Token)
- ITL (Inter-Token Latency)
- TPS (Tokens Per Second)
- Queue depth and utilization

See [pkg/analyzer/README.md](../../pkg/analyzer/README.md) for detailed documentation.

## Data Flow

### Reconciliation Flow

```
1. Event Trigger
   └──► Controller receives VariantAutoscaling event
        └──► Engine.Reconcile(ctx, va)
             
2. Metrics Collection
   └──► Collector.FetchMetrics(modelID, namespace)
        └──► Query Prometheus for vLLM metrics
             └──► Return ReplicaMetrics[]

3. Saturation Analysis
   └──► Analyzer.AnalyzeModelSaturation(metrics, config)
        └──► Calculate spare capacity
             └──► Determine scale-up/down needs
                  └──► Return ModelSaturationAnalysis

4. Target Calculation
   └──► Analyzer.CalculateCapacityTargets(analysis, variants)
        └──► Per-variant target replicas
             └──► Cost-aware selection
                  └──► Return map[variantName]targetReplicas

5. Optimization (Optional)
   └──► Optimizer.Optimize(system, targets)
        └──► Queueing model validation
             └──► SLO compliance check
                  └──► Return optimized allocations

6. Actuation
   └──► Actuator.EmitMetrics(targets)
        └──► Update deployment replicas (if enabled)
             └──► Update CR status
                  └──► Record Kubernetes events
```

### Metric Collection Flow

```
Prometheus Query ──► Cache Check ──► Fresh? ──Yes──► Return Cached
                           │
                           No
                           │
                           ▼
                    Execute Query ──► Parse Results ──► Enrich Metadata
                           │
                           ▼
                    Update Cache ──► Return Metrics
```

## Key Interfaces

### MetricsCollector (`internal/interfaces/metrics_collector.go`)

```go
type MetricsCollector interface {
    FetchSaturationMetrics(ctx, modelID, namespace) ([]ReplicaMetrics, error)
    ValidateMetricsAvailability(ctx, modelID, namespace) error
}
```

### SaturationAnalyzer (`internal/interfaces/saturation_analyzer.go`)

```go
type SaturationAnalyzer interface {
    AnalyzeModelSaturation(ctx, modelID, namespace, metrics, config) (*ModelSaturationAnalysis, error)
    CalculateCapacityTargets(analysis, variants) (map[string]int, error)
    ArbitrateWithModelBased(capacityTargets, modelBasedTargets) ([]VariantDecision, error)
}
```

### VariantAutoscalingsEngine (`internal/interfaces/interfaces.go`)

```go
type VariantAutoscalingsEngine interface {
    Reconcile(ctx, va) error
    GetRecorder() record.EventRecorder
}
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PROMETHEUS_URL` | - | Prometheus server URL (required) |
| `PROMETHEUS_CA_CERT_PATH` | - | Path to Prometheus CA certificate |
| `METRICS_POLL_INTERVAL` | `30s` | Metrics collection interval |
| `RECONCILE_INTERVAL` | `60s` | Controller reconciliation interval |
| `SATURATION_KV_CACHE_THRESHOLD` | `80.0` | KV-cache trigger (%) |
| `SATURATION_QUEUE_DEPTH_THRESHOLD` | `5` | Queue depth trigger |
| `SATURATION_SPARE_CAPACITY_TARGET` | `20.0` | Target spare capacity (%) |
| `SATURATION_SAFETY_MARGIN` | `10.0` | Scale-down safety buffer (%) |

### CRD Configuration

See [CRD Reference](../user-guide/crd-reference.md) for complete API documentation.

**Example:**
```yaml
apiVersion: llmd.ai/v1alpha1
kind: VariantAutoscaling
metadata:
  name: llama-8b-autoscaler
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: llama-8b-vllm
  modelID: "meta/llama-3.1-8b"
  variantCost: "10.0"
```

## Design Principles

### Modularity
- Clear separation between collection, analysis, optimization, and actuation
- Pluggable backends (Prometheus, EPP)
- Swappable engines (saturation, model-based)

### Safety-First
- Conservative scaling decisions with safety margins
- Worst-case simulation before scale-down
- Metric validation and staleness checks

### Cost-Awareness
- Per-variant cost tracking
- Prefer cheaper variants when capacity allows
- Cost-optimal scaling recommendations

### Progressive Enhancement
- Basic functionality without optimization layer
- Optional hybrid mode with queueing models
- Gradual rollout of advanced features

### Observability
- Comprehensive metric emission
- Kubernetes event recording
- Structured logging with context

## Extension Points

### Adding a New Engine

1. Implement `VariantAutoscalingsEngine` interface
2. Add factory method in `internal/controller/`
3. Update controller to support engine selection
4. Document configuration and behavior

### Adding a New Metrics Backend

1. Implement `MetricsCollector` interface
2. Add backend in `internal/collector/`
3. Update factory in `internal/collector/factory.go`
4. Add configuration support

### Adding Custom Analyzers

1. Implement `SaturationAnalyzer` interface
2. Add analyzer in `internal/saturation/`
3. Wire into engine reconciliation flow
4. Document algorithm and configuration

## Related Documentation

- [Saturation Analyzer](../saturation-analyzer.md) - Detailed saturation analysis algorithm
- [Saturation Scaling Configuration](../saturation-scaling-config.md) - Configuration guide
- [Modeling & Optimization](modeling-optimization.md) - Queueing theory and optimization
- [Metrics & Monitoring](../metrics-health-monitoring.md) - Observability guide
- [Development Guide](../developer-guide/development.md) - Contributing and testing

## References

- [WVA Proposal](https://docs.google.com/document/d/1n6SAhloQaoSyF2k3EveIOerT-f97HuWXTLFm07xcvqk/edit)
- [Saturation Design Discussion](https://docs.google.com/document/d/1iGHqdxRUDpiKwtJFr5tMCKM7RF6fbTfZBL7BTn6UkwA/edit?tab=t.0#heading=h.mdte0lq44ul4)
- [API Proposal](https://docs.google.com/document/d/1j2KRAT68_FYxq1iVzG0xVL-DHQhGVUZBqiM22Hd_0hc/edit)
