# Workload-Variant-Autoscaler Architecture

This document describes the architecture and component interactions of the Workload-Variant-Autoscaler (WVA).

## Overview

WVA is a Kubernetes controller that performs intelligent autoscaling for inference model servers based on **saturation analysis**. It monitors vLLM server metrics, analyzes capacity utilization, and determines optimal replica counts to prevent capacity exhaustion while minimizing infrastructure costs.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Kubernetes Cluster                       │
│                                                                  │
│  ┌──────────────┐        ┌──────────────┐                      │
│  │ vLLM Pods    │───────▶│  Prometheus  │                      │
│  │ (Inference)  │ metrics│              │                      │
│  └──────────────┘        └───────┬──────┘                      │
│         ▲                         │                             │
│         │                         │ metrics                     │
│         │ scale                   ▼                             │
│  ┌──────┴───────────────────────────────────────┐              │
│  │         WVA Controller                       │              │
│  │  ┌──────────┐  ┌──────────┐  ┌───────────┐  │              │
│  │  │Collector │─▶│ Analyzer │─▶│ Actuator  │  │              │
│  │  └──────────┘  └──────────┘  └───────────┘  │              │
│  │       ▲             │              │         │              │
│  │       │             ▼              ▼         │              │
│  │       │        [Analysis]    [Metrics Out]  │              │
│  │       │                                      │              │
│  │  ┌────┴──────────────────────────┐          │              │
│  │  │    Reconciler                 │          │              │
│  │  │ (VariantAutoscaling watcher)  │          │              │
│  │  └───────────────────────────────┘          │              │
│  └──────────────────────────────────────────────┘              │
│         │                         ▲                             │
│         │ desired_replicas        │ HPA reads                  │
│         ▼ metric                  │ metric                     │
│  ┌──────────────┐        ┌────────┴──────┐                    │
│  │  Prometheus  │───────▶│      HPA      │                    │
│  │   (metrics)  │        │   or KEDA     │                    │
│  └──────────────┘        └───────────────┘                    │
│                                   │                             │
│                                   │ scales                      │
│                                   ▼                             │
│                          ┌──────────────┐                      │
│                          │  Deployment  │                      │
│                          │  (vLLM pods) │                      │
│                          └──────────────┘                      │
└─────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Reconciler (`internal/controller`)

The Reconciler is the main Kubernetes controller that watches `VariantAutoscaling` custom resources.

**Responsibilities:**
- Watch VariantAutoscaling CRs for changes
- Watch related Deployments for status updates
- Trigger the scaling analysis pipeline on reconciliation events
- Update VariantAutoscaling status with current and desired allocations
- Emit Kubernetes events for important state changes

**Key Files:**
- `variantautoscaling_controller.go` - Main reconciliation loop
- `predicates.go` - Event filtering logic
- `allocation.go` - Allocation tracking helpers

**Reconciliation Triggers:**
- VariantAutoscaling CR created/updated/deleted
- Target Deployment status changes
- ServiceMonitor changes
- Periodic resync (default: 60s)

### 2. Metrics Collector (`internal/collector`)

The Collector gathers metrics from Prometheus and cluster state from Kubernetes.

**Responsibilities:**
- Query Prometheus for vLLM saturation metrics (KV cache usage, queue length)
- Enrich metrics with pod metadata (variant name, accelerator type)
- Query Kubernetes API for deployment and pod status
- Support multiple backends (Prometheus primary, EPP experimental)
- Implement caching and background fetching for efficiency

**Key Files:**
- `collector.go` - Legacy compatibility functions
- `factory.go` - Collector instantiation
- `prometheus/prometheus_collector.go` - Prometheus implementation
- `prometheus/saturation_metrics.go` - Saturation metric queries
- `prometheus/background_fetching.go` - Background metric fetching
- `cache/memory_cache.go` - In-memory metric caching

**Metrics Collected:**
- `vllm:kv_cache_usage_perc` - KV cache utilization percentage
- `vllm:num_requests_waiting` - Number of queued requests
- Pod readiness and phase information

### 3. Saturation Analyzer (`internal/saturation`)

The Analyzer performs saturation-based capacity analysis to determine scaling decisions.

**Responsibilities:**
- Analyze replica metrics to identify saturated vs. non-saturated servers
- Calculate spare capacity across all replicas
- Determine scale-up triggers (capacity exhaustion risk)
- Validate scale-down safety (worst-case simulation)
- Make per-variant scaling decisions with cost-awareness
- Support capacity-only mode and hybrid mode with model-based optimization

**Key Files:**
- `analyzer.go` - Core saturation analysis logic
- `constants.go` - Threshold constants

**Analysis Algorithm:**
1. **Classify replicas**: Saturated vs. non-saturated based on thresholds
2. **Calculate spare capacity**: Average remaining capacity across non-saturated replicas
3. **Scale-up decision**: Trigger if spare capacity < trigger threshold
4. **Scale-down safety**: Simulate worst-case scenario to validate safety
5. **Per-variant allocation**: Cost-aware replica distribution

**Configuration:**
- `kvCacheThreshold`: 80% (scale if KV cache > 80%)
- `queueLengthThreshold`: 5 requests (scale if queue > 5)
- `triggerThreshold`: 20% (scale up if spare capacity < 20%)
- `targetThreshold`: 40% (target 40% spare capacity after scale-up)

See [Saturation Analyzer Documentation](saturation-analyzer.md) for detailed algorithm description.

### 4. Scaling Engines (`internal/engines`)

Engines implement different scaling strategies and execution patterns.

**Available Engines:**

**Saturation Engine** (`engines/saturation/`):
- Primary scaling engine using saturation-based analysis
- Integrates with Saturation Analyzer
- Supports polling-based execution
- Handles metrics validation and staleness checks

**Model Engine** (`engines/model/`):
- Experimental queueing-theory based optimization
- Uses M/M/1/k and M/G/1 models for latency prediction
- Requires offline profiling for model parameters
- Currently disabled in production (future work)

**Scale-from-Zero Engine** (`engines/scalefromzero/`):
- Handles cold-start scenarios
- Scales deployments from 0 to 1 replica when needed

**Common Infrastructure** (`engines/common/`):
- Shared caching layer for engine state
- Common utilities and helpers

### 5. Actuator (`internal/actuator`)

The Actuator emits metrics to Prometheus for consumption by external autoscalers (HPA/KEDA).

**Responsibilities:**
- Emit `inferno_desired_replicas` metric with variant-specific labels
- Expose metrics endpoint for Prometheus scraping
- Track optimization status and metadata

**Key Files:**
- `actuator.go` - Metrics emission and status tracking

**Emitted Metrics:**
- `inferno_desired_replicas{variant_name="...", model_id="...", accelerator="..."}` - Target replica count

**Integration:**
- HPA/KEDA reads `inferno_desired_replicas` via Prometheus Adapter
- HPA/KEDA scales the target Deployment based on this metric

### 6. Optimizer and Solver (`internal/optimizer`, `pkg/solver`)

The Optimizer performs global optimization across multiple variants and models.

**Responsibilities:**
- Coordinate between different scaling engines
- Integrate saturation-based and model-based decisions
- Handle resource allocation in unlimited vs. limited capacity modes
- Solve allocation problems using greedy or optimization algorithms

**Key Files:**
- `internal/optimizer/optimizer.go` - High-level optimization coordination
- `pkg/solver/solver.go` - Allocation problem solver
- `pkg/solver/greedy.go` - Greedy allocation algorithm

**Operating Modes:**
- **Unlimited mode** (current): Each variant gets optimal allocation independently
- **Limited mode** (future): Global optimization with cluster capacity constraints

### 7. Core Domain Models (`pkg/core`)

Core domain models represent the system's entities and their relationships.

**Key Abstractions:**
- `System` - Global state (accelerators, models, servers, capacity)
- `Server` - Inference server instance with allocation
- `Allocation` - GPU and replica allocation for a server
- `Model` - LLM model with performance characteristics
- `Accelerator` - GPU type with cost and availability
- `ServiceClass` - QoS class with priorities and SLOs

**Key Files:**
- `system.go` - System-wide state management
- `server.go` - Server and allocation logic
- `allocation.go` - Allocation calculations
- `model.go` - Model metadata

### 8. Queue Analyzer (`pkg/analyzer`)

The Queue Analyzer provides queueing-theory based performance modeling (experimental).

**Responsibilities:**
- Model inference server behavior using M/M/1/k and state-dependent models
- Estimate latency (TTFT, ITL) and throughput given load and configuration
- Size servers to meet SLO targets (inverse analysis)

**Key Files:**
- `queueanalyzer.go` - Main analyzer interface
- `queuemodel.go` - Queueing model implementation
- `mm1kmodel.go` - M/M/1/k model
- `mm1modelstatedependent.go` - State-dependent M/M/1 model

**Note:** This component is currently not used in production. The current implementation relies on saturation-based scaling instead of queueing-theory based optimization.

## Data Flow

### 1. Reconciliation Flow

```
VariantAutoscaling CR Event
         │
         ▼
    Reconciler
         │
         ├──▶ Validate CR spec
         │
         ├──▶ Collector.CollectMetrics()
         │         │
         │         ├──▶ Query Prometheus (KV cache, queue length)
         │         ├──▶ Query Kubernetes (pods, deployments)
         │         └──▶ Return ReplicaMetrics[]
         │
         ├──▶ SaturationAnalyzer.Analyze()
         │         │
         │         ├──▶ Classify saturated replicas
         │         ├──▶ Calculate spare capacity
         │         ├──▶ Determine scale-up/down decisions
         │         └──▶ Return ModelSaturationAnalysis
         │
         ├──▶ Engine.Execute()
         │         │
         │         ├──▶ Process analysis results
         │         ├──▶ Apply cost-aware variant selection
         │         └──▶ Return allocation decisions
         │
         ├──▶ Actuator.EmitMetrics()
         │         │
         │         └──▶ Publish inferno_desired_replicas
         │
         └──▶ Update VariantAutoscaling Status
                   │
                   ├──▶ currentReplicas
                   ├──▶ desiredReplicas
                   ├──▶ conditions
                   └──▶ optimization metadata
```

### 2. Metrics Flow

```
vLLM Pods
    │ expose /metrics endpoint
    │
    ▼
Prometheus
    │ scrape via ServiceMonitor
    │
    ▼
WVA Collector
    │ query with PromQL
    │
    ▼
Saturation Analyzer
    │ analyze capacity
    │
    ▼
Actuator
    │ emit desired_replicas
    │
    ▼
Prometheus
    │ scrape WVA metrics endpoint
    │
    ▼
Prometheus Adapter
    │ expose as external metric
    │
    ▼
HPA/KEDA
    │ scale deployment
    │
    ▼
vLLM Deployment
    │ reconcile to desired replicas
    │
    ▼
vLLM Pods (scaled)
```

## Configuration

### VariantAutoscaling CRD

The `VariantAutoscaling` custom resource defines autoscaling configuration for a model variant.

**Key Fields:**
- `spec.modelID` - HuggingFace model identifier
- `spec.scaleTargetRef` - Target deployment to scale
- `spec.modelProfile.accelerators[]` - Per-accelerator performance parameters
- `status.currentReplicas` - Current replica count from deployment
- `status.desiredReplicas` - Calculated desired replica count
- `status.conditions` - Status conditions (MetricsAvailable, OptimizationComplete, etc.)

See [CRD Reference](user-guide/crd-reference.md) for complete field documentation.

### Controller Configuration

Controller configuration is provided via ConfigMaps and environment variables:

**ConfigMaps:**
- `workload-variant-autoscaler-config` - Core controller settings
- `service-classes` - QoS class definitions
- `model-accelerator-data` - Accelerator costs and availability

**Environment Variables:**
- `PROMETHEUS_URL` - Prometheus API endpoint
- `PROMETHEUS_CA_CERT_PATH` - TLS CA certificate path
- `PROMETHEUS_SKIP_TLS_VERIFY` - Skip TLS verification (dev only)
- `RECONCILE_INTERVAL` - Reconciliation period (default: 60s)
- `LOG_LEVEL` - Logging verbosity (debug, info, warn, error)

### Saturation Configuration

Saturation thresholds are configurable via the `saturation-scaling-config` ConfigMap:

```yaml
saturation:
  kvCacheThreshold: 0.80        # Scale if KV cache > 80%
  queueLengthThreshold: 5       # Scale if queue > 5 requests
  triggerThreshold: 0.20        # Scale up if spare capacity < 20%
  targetThreshold: 0.40         # Target 40% spare capacity
```

## Execution Patterns

### Polling Executor

The current implementation uses a polling-based execution pattern:

- **Periodic reconciliation**: Controller reconciles on a fixed interval (default: 60s)
- **Event-driven reconciliation**: Triggered by CR updates or deployment changes
- **Metrics staleness handling**: Validates metrics freshness before analysis

### Future: Reactive Executor

Planned enhancement for event-driven scaling:

- **Metrics-driven triggers**: Scale immediately when saturation detected
- **Webhook integration**: React to external events
- **Adaptive polling**: Adjust reconciliation frequency based on load

## Operating Modes

### Current: Saturation-Only Mode

The current implementation uses pure saturation-based scaling:

- Analyzes live vLLM metrics (no offline profiling required)
- Makes per-variant scaling decisions
- Cost-aware replica allocation
- Fast and reactive

### Future: Hybrid Mode

Planned enhancement combining saturation and model-based approaches:

- Saturation analysis for safety guardrails
- Queueing-theory models for SLO optimization
- Arbitration matrix for decision conflicts
- Fallback to saturation when model confidence low

## Testing

### Unit Tests

- Package-level tests in `*_test.go` files
- Mock interfaces for external dependencies
- Table-driven test patterns

### Integration Tests

- `test/e2e/` - Basic end-to-end scenarios
- `test/e2e-saturation-based/` - Saturation-specific scenarios
- `test/e2e-openshift/` - OpenShift-specific tests

### Test Utilities

- `test/utils/e2eutils.go` - E2E test helpers
- `test/utils/unitutils.go` - Unit test utilities
- Suite-based test organization using Ginkgo

## Observability

### Metrics

**WVA Controller Metrics:**
- `inferno_desired_replicas` - Target replica count per variant
- Standard controller-runtime metrics (reconciliation time, errors, etc.)

**vLLM Server Metrics (consumed):**
- `vllm:kv_cache_usage_perc` - KV cache utilization
- `vllm:num_requests_waiting` - Queue length
- `vllm:num_requests_running` - Active requests

### Logging

Structured logging using controller-runtime logging framework:

- **INFO**: Normal operations, reconciliation events
- **DEBUG**: Detailed metrics, analysis steps
- **ERROR**: Failures, retryable errors
- **WARN**: Degraded conditions, stale metrics

### Events

Kubernetes events emitted for important state changes:

- `OptimizationComplete` - Successful scaling decision
- `MetricsUnavailable` - Cannot collect metrics
- `ScaleDecisionFailed` - Analysis error

## Security

### RBAC

WVA requires the following permissions:

- **VariantAutoscaling**: get, list, watch, update, patch
- **Deployments**: get, list, watch
- **Pods**: get, list, watch
- **ServiceMonitors**: get, list, watch
- **ConfigMaps**: get, list, watch
- **Events**: create, patch

See [charts/workload-variant-autoscaler/templates/rbac/](../charts/workload-variant-autoscaler/templates/rbac/) for complete RBAC configuration.

### TLS

Prometheus connections support TLS with configurable CA certificate verification:

- Mount CA certificate as Secret or ConfigMap
- Configure `PROMETHEUS_CA_CERT_PATH`
- Set `PROMETHEUS_SKIP_TLS_VERIFY=true` for dev/testing only

## Performance Considerations

### Caching

- **Metrics caching**: Background fetching reduces reconciliation latency
- **State caching**: Engine-level caching for allocation state
- **TTL-based invalidation**: Configurable cache expiration

### Scalability

- **Single controller instance**: Active/standby leader election
- **Per-variant analysis**: Independent scaling decisions
- **Batch metrics queries**: Efficient Prometheus queries with label selectors

### Resource Usage

- **CPU**: Low during steady-state, spikes during reconciliation
- **Memory**: Proportional to number of variants and metrics cache size
- **Network**: Periodic Prometheus queries (default: every 60s)

## Future Enhancements

### Planned Features

1. **Reactive execution**: Event-driven scaling triggers
2. **Hybrid optimization**: Combine saturation and model-based approaches
3. **Limited capacity mode**: Global optimization with cluster constraints
4. **Multi-model coordination**: Cross-model resource allocation
5. **Predictive scaling**: Proactive scaling based on traffic forecasts
6. **GPU disaggregation support**: Prefill/decode separation
7. **Cost optimization**: Multi-objective optimization (cost + SLOs)

### Experimental Features

- **EPP integration**: Alternative metrics backend
- **Custom metrics**: User-defined saturation indicators
- **Advanced queueing models**: More sophisticated performance prediction

## References

- [Saturation Analyzer Details](saturation-analyzer.md)
- [User Guide](user-guide/installation.md)
- [CRD Reference](user-guide/crd-reference.md)
- [Developer Guide](developer-guide/development.md)
- [Deployment Guide](../deploy/README.md)
