# Internal Package Reference

This document provides a comprehensive reference for all internal packages in the Workload Variant Autoscaler (WVA) codebase.

## Package Organization

```
internal/
├── actuator/          # Scaling actuation and metrics emission
├── collector/         # Metrics collection from external sources
│   ├── cache/        # Metric caching subsystem
│   ├── config/       # Collector configuration
│   └── prometheus/   # Prometheus backend implementation
├── config/            # Global configuration management
├── constants/         # Shared constants and metric names
├── controller/        # Kubernetes controller implementation
├── discovery/         # Cluster resource discovery
├── engines/           # Autoscaling engines
│   ├── common/       # Shared engine utilities
│   ├── executor/     # Task execution strategies
│   ├── model/        # Queueing model-based engine
│   ├── saturation/   # Saturation-based engine (primary)
│   └── scalefromzero/# Cold-start handling
├── interfaces/        # Core interfaces and contracts
├── logging/           # Logging utilities
├── metrics/           # Prometheus metrics registration
├── modelanalyzer/     # Model analysis utilities
├── optimizer/         # Global optimization logic
├── saturation/        # Saturation analysis algorithms
└── utils/             # Common utilities

pkg/
├── analyzer/          # Queueing theory models (PUBLIC)
├── config/            # Configuration types (PUBLIC)
├── core/              # Core domain models (PUBLIC)
├── manager/           # High-level management API (PUBLIC)
└── solver/            # Optimization solvers (PUBLIC)
```

## Internal Packages

### `internal/actuator`

**Purpose:** Executes scaling decisions by emitting metrics and updating deployments.

**Key Responsibilities:**
- Emit custom metrics to Prometheus for HPA/KEDA consumption
- Update deployment replica counts (when direct actuation enabled)
- Record Kubernetes events for observability
- Update VariantAutoscaling CR status

**Metrics Emitted:**
- `wva_desired_replicas` - Target replica count per variant
- `wva_current_replicas` - Observed replica count
- `wva_saturation_level` - Current KV-cache/queue metrics
- `wva_scaling_decision` - Scale direction indicator

**Usage:**
```go
actuator := actuator.New(client, recorder, metricsRegistry)
err := actuator.EmitMetrics(ctx, targets, currentState)
```

---

### `internal/collector`

**Purpose:** Pluggable metrics collection system with support for multiple backends.

**Architecture:**
```
MetricsCollector (interface)
    ├── PrometheusCollector
    └── (future: EPPCollector)
```

**Factory Pattern:**
```go
collector := factory.NewMetricsCollector(
    factory.PrometheusBackend,
    promClient,
    k8sClient,
)
```

**Subpackages:**

#### `internal/collector/cache`
Implements metric caching to reduce backend query load.

**Types:**
- `Cache` - Generic cache interface
- `MemoryCache` - In-memory TTL-based cache
- `NoopCache` - Null object for testing

**Configuration:**
- TTL (time-to-live) for cached entries
- Max size limits
- Eviction policies

#### `internal/collector/config`
Configuration types and validation for collectors.

**Key Types:**
```go
type CollectorConfig struct {
    Backend         string
    PrometheusURL   string
    CACertPath      string
    QueryTimeout    time.Duration
    CacheTTL        time.Duration
}
```

#### `internal/collector/prometheus`
Prometheus-specific implementation of MetricsCollector.

**Key Files:**
- `prometheus_collector.go` - Main collector implementation
- `saturation_metrics.go` - Saturation metric queries
- `background_fetching.go` - Async metric fetching
- `cache_operations.go` - Cache integration
- `query_helpers.go` - PromQL utilities
- `tracking.go` - Query result tracking

**Metrics Queried:**
```promql
# KV-cache utilization (peak over 1 minute)
max_over_time(vllm:gpu_cache_usage_perc{model_name="$modelID"}[1m])

# Queue depth (peak over 1 minute)
max_over_time(vllm:num_requests_waiting{model_name="$modelID"}[1m])

# Request rate
rate(vllm:request_success_total{model_name="$modelID"}[1m])
```

**Features:**
- Background metric fetching for improved latency
- Automatic retry with exponential backoff
- Metric staleness detection
- Pod metadata enrichment (variant name, accelerator type)

---

### `internal/config`

**Purpose:** Global configuration management for WVA.

**Key Files:**
- `prometheus.go` - Prometheus client configuration
- `helpers.go` - Configuration utilities

**Configuration Sources:**
1. Environment variables
2. ConfigMaps
3. Command-line flags
4. Default values

---

### `internal/constants`

**Purpose:** Centralized constant definitions.

**Key Files:**
- `metrics.go` - Metric names and labels

**Constants:**
```go
const (
    // vLLM Metrics
    VLLMKvCacheUsagePerc     = "vllm:gpu_cache_usage_perc"
    VLLMNumRequestsWaiting   = "vllm:num_requests_waiting"
    VLLMRequestSuccessTotal  = "vllm:request_success_total"
    
    // WVA Metrics
    WVADesiredReplicas       = "wva_desired_replicas"
    WVACurrentReplicas       = "wva_current_replicas"
    WVASaturationLevel       = "wva_saturation_level"
)
```

---

### `internal/controller`

**Purpose:** Kubernetes controller implementation for VariantAutoscaling CRD.

**Key Files:**
- `variantautoscaling_controller.go` - Main reconciler
- `predicates.go` - Event filtering logic
- `allocation.go` - Allocation helper functions

**Reconciliation Flow:**
```go
func (r *Reconciler) Reconcile(ctx, req) (ctrl.Result, error) {
    // 1. Fetch VariantAutoscaling CR
    // 2. Validate configuration
    // 3. Delegate to engine
    // 4. Update status
    // 5. Requeue if needed
}
```

**Predicates:**
- Filter irrelevant events (metadata-only updates)
- Watch related resources (Deployments, Pods)
- Trigger reconciliation on metric changes

---

### `internal/discovery`

**Purpose:** Discover cluster resources and accelerator inventory.

**Implementations:**
- `K8sWithGpuOperator` - Uses NVIDIA GPU Operator labels

**Discovery Process:**
1. Query nodes with accelerator labels
2. Parse GPU types and counts
3. Build inventory map: `map[nodeGroup]map[gpuType]count`

**Future:** Support for AMD, Intel, and custom accelerators.

---

### `internal/engines`

**Purpose:** Pluggable autoscaling engines with different strategies.

#### `internal/engines/saturation`
Primary engine using capacity-based autoscaling.

**Algorithm:**
1. Collect saturation metrics (KV-cache, queue depth)
2. Analyze spare capacity across all replicas
3. Calculate per-variant target replicas
4. Optionally arbitrate with queueing model
5. Emit metrics and update status

**Configuration:**
```go
type SaturationConfig struct {
    KvCacheThreshold      float64  // 80.0
    QueueDepthThreshold   int      // 5
    SpareCapacityTarget   float64  // 20.0
    SafetyMargin          float64  // 10.0
}
```

**Thread Safety:** Safe for concurrent use.

#### `internal/engines/model`
Queueing theory-based engine (alternative to saturation).

**Models:**
- M/M/1/k - Markovian queue with capacity
- M/G/1 - General service time

**Use Cases:**
- Predictive scaling based on forecasted load
- SLO-aware optimization (TTFT, ITL targets)
- Capacity planning and "what-if" analysis

#### `internal/engines/scalefromzero`
Handles cold-start scenarios when replicas are at zero.

**Strategy:**
- Start with minimum replicas (typically 1)
- Monitor readiness and metric availability
- Transition to normal autoscaling mode

#### `internal/engines/executor`
Task execution strategies for optimization.

**Types:**
- `PollingExecutor` - Fixed-interval execution
- `ReactiveExecutor` - Event-driven execution (TODO)
- `HybridExecutor` - Combined approach (TODO)

**Usage:**
```go
executor := executor.NewPollingExecutor(interval)
executor.Start(ctx, optimizationFunc)
```

#### `internal/engines/common`
Shared utilities for all engines.

**Key Features:**
- Result caching
- State management
- Error handling patterns

---

### `internal/interfaces`

**Purpose:** Define core contracts and data structures.

**Key Interfaces:**

#### `MetricsCollector`
```go
type MetricsCollector interface {
    FetchSaturationMetrics(ctx, modelID, namespace) ([]ReplicaMetrics, error)
    ValidateMetricsAvailability(ctx, modelID, namespace) error
}
```

#### `SaturationAnalyzer`
```go
type SaturationAnalyzer interface {
    AnalyzeModelSaturation(ctx, modelID, namespace, metrics, config) (*ModelSaturationAnalysis, error)
    CalculateCapacityTargets(analysis, variants) (map[string]int, error)
}
```

#### `VariantAutoscalingsEngine`
```go
type VariantAutoscalingsEngine interface {
    Reconcile(ctx, va) error
    GetRecorder() record.EventRecorder
}
```

**Key Data Structures:**
- `ReplicaMetrics` - Per-replica saturation metrics
- `ModelSaturationAnalysis` - Aggregate analysis results
- `VariantDecision` - Per-variant scaling decision
- `SaturationScalingConfig` - Configuration parameters

---

### `internal/logging`

**Purpose:** Structured logging utilities.

**Features:**
- Context-aware logging
- Correlation IDs
- Leveled logging (Debug, Info, Warn, Error)
- Testing helpers

**Usage:**
```go
log := logging.FromContext(ctx)
log.Info("Reconciling variant", "modelID", modelID)
```

---

### `internal/metrics`

**Purpose:** Prometheus metrics registration.

**Metrics Types:**
- Gauges (current values)
- Counters (cumulative)
- Histograms (distributions)

**Registration:**
```go
registry := metrics.NewRegistry()
registry.MustRegister(desiredReplicasGauge)
```

---

### `internal/optimizer`

**Purpose:** Global optimization across variants.

**Current Mode:** Unlimited
- Each variant receives optimal allocation independently
- No cluster capacity constraints
- Compatible with cluster autoscalers

**Algorithm:**
1. For each variant, query model analyzer for minimum replicas
2. Calculate cost-optimal allocation
3. Emit optimization metrics
4. Return target allocations

**Future:** Limited mode with capacity-aware allocation.

---

### `internal/saturation`

**Purpose:** Core saturation analysis algorithms.

**Key File:** `analyzer.go`

**Main Functions:**

#### `AnalyzeModelSaturation()`
Analyzes all replicas across all variants of a model.

**Steps:**
1. Aggregate metrics from all replicas
2. Identify non-saturated replicas
3. Calculate average spare capacity
4. Determine scale-up/down needs
5. Perform safety simulation for scale-down

**Returns:**
```go
type ModelSaturationAnalysis struct {
    ModelID              string
    TotalReplicas        int
    SaturatedReplicas    int
    AverageSpareCapacity float64
    ShouldScaleUp        bool
    ShouldScaleDown      bool
    ScaleDownSafe        bool
    VariantAnalyses      []VariantSaturationAnalysis
}
```

#### `CalculateCapacityTargets()`
Determines target replica counts per variant.

**Algorithm:**
1. If scale-up needed: Add replicas to lowest-cost variant
2. If scale-down safe: Remove from highest-cost variant
3. Preserve desired replicas from CRD
4. Ensure minimum of 1 replica per variant

**Cost-Awareness:**
```go
// Prefer variants with lower cost when scaling up
cheapestVariant := findLowestCost(variants)
targets[cheapestVariant] += 1
```

---

### `internal/utils`

**Purpose:** Common utilities and helper functions.

**Key Files:**
- `allocation.go` - Allocation data structure helpers
- `variant.go` - Variant name parsing and generation
- `tls.go` - TLS configuration for Prometheus
- `prometheus_transport.go` - HTTP transport with CA cert
- `utils.go` - General utilities

**Utilities:**
- String parsing and validation
- Float/int conversions with defaults
- Kubernetes label selectors
- Error wrapping and context

---

## Public Packages (pkg/)

### `pkg/analyzer`

**Purpose:** Queueing theory models for performance analysis.

**Models:**
- `MM1KModel` - M/M/1/k queueing model
- `MM1StateDependent` - State-dependent variant
- `QueueAnalyzer` - High-level analysis API

See [pkg/analyzer/README.md](../../pkg/analyzer/README.md) for details.

---

### `pkg/config`

**Purpose:** Configuration types and defaults.

**Key Types:**
```go
type Config struct {
    PrometheusURL        string
    ReconcileInterval    time.Duration
    MetricsPollInterval  time.Duration
    SaturationConfig     SaturationScalingConfig
}
```

---

### `pkg/core`

**Purpose:** Core domain models.

**Key Types:**
- `Accelerator` - GPU/accelerator representation
- `Allocation` - Resource allocation
- `Model` - Model metadata and profile
- `Server` - Inference server state
- `ServiceClass` - SLO definitions
- `System` - Global system state

---

### `pkg/solver`

**Purpose:** Optimization solvers.

**Implementations:**
- `GreedySolver` - Fast greedy optimization
- (future) `MIPSolver` - Mixed-integer programming

**Usage:**
```go
solver := solver.NewGreedySolver()
solution := solver.Solve(system, constraints)
```

---

## Development Guidelines

### Adding New Packages

1. **Internal Packages** (`internal/`):
   - For WVA-specific implementation details
   - Not importable by external code
   - Free to change without breaking compatibility

2. **Public Packages** (`pkg/`):
   - For reusable libraries and APIs
   - Semantic versioning applies
   - Document exported types and functions

### Package Documentation

Each package should include:
1. `doc.go` with package overview
2. README.md for complex packages
3. Inline comments for exported types
4. Example usage in tests

### Import Guidelines

```go
// Standard library
import (
    "context"
    "fmt"
)

// External dependencies
import (
    "k8s.io/client-go/kubernetes"
    "sigs.k8s.io/controller-runtime/pkg/client"
)

// Internal packages
import (
    "github.com/llm-d-incubation/workload-variant-autoscaler/api/v1alpha1"
    "github.com/llm-d-incubation/workload-variant-autoscaler/internal/collector"
    "github.com/llm-d-incubation/workload-variant-autoscaler/pkg/analyzer"
)
```

---

## Related Documentation

- [Architecture Overview](architecture.md) - System design
- [Development Guide](../developer-guide/development.md) - Contributing
- [Testing Guide](../developer-guide/testing.md) - Testing practices

---

## Quick Reference

| Task | Package |
|------|---------|
| Collect metrics | `internal/collector` |
| Analyze saturation | `internal/saturation` |
| Queueing models | `pkg/analyzer` |
| Optimize allocations | `internal/optimizer` |
| Emit metrics | `internal/actuator` |
| Kubernetes controller | `internal/controller` |
| Configuration | `internal/config`, `pkg/config` |
| Logging | `internal/logging` |
| Utilities | `internal/utils` |
