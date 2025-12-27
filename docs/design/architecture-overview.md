# Architecture Overview

## Introduction

The Workload-Variant-Autoscaler (WVA) is a Kubernetes controller that performs intelligent autoscaling for inference model servers based on saturation metrics. This document provides a comprehensive overview of WVA's architecture, components, and data flow.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Kubernetes Cluster                          │
│                                                                     │
│  ┌────────────────┐         ┌──────────────────┐                  │
│  │  vLLM Pods     │────────▶│   Prometheus     │                  │
│  │  (Inference    │ metrics │   (Monitoring)   │                  │
│  │   Servers)     │         │                  │                  │
│  └────────────────┘         └────────┬─────────┘                  │
│         ▲                             │                            │
│         │                             │ query metrics              │
│         │ scale                       ▼                            │
│  ┌──────┴─────────────────────────────────────────────────────┐   │
│  │         Workload-Variant-Autoscaler (WVA)                  │   │
│  │                                                             │   │
│  │  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌─────────┐│   │
│  │  │Collector │──▶│ Analyzer │──▶│Optimizer │──▶│Actuator ││   │
│  │  └──────────┘   └──────────┘   └──────────┘   └─────────┘│   │
│  │       │              │               │              │      │   │
│  │       └──────────────┴───────────────┴──────────────┘      │   │
│  │                   Reconciliation Loop                      │   │
│  └─────────────────────────────────────────────────────────────┘   │
│         │                             │                            │
│         │ emit metrics                │ scale decisions            │
│         ▼                             ▼                            │
│  ┌────────────────┐         ┌──────────────────┐                  │
│  │  Prometheus    │         │   HPA / KEDA     │                  │
│  │  (Custom       │         │   (External      │                  │
│  │   Metrics)     │         │    Scaler)       │                  │
│  └────────────────┘         └──────────────────┘                  │
└─────────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Controller

**Location:** `internal/controller/variantautoscaling_controller.go`

The Kubernetes controller orchestrates the reconciliation loop and manages VariantAutoscaling custom resources.

**Responsibilities:**
- Watch VariantAutoscaling CRDs for changes
- Trigger reconciliation on events (CR changes, periodic sync)
- Coordinate component execution
- Update CR status with optimization results
- Handle error conditions and retries

**Key Features:**
- Event-driven reconciliation with configurable sync period (default: 60s)
- Predicate filtering to optimize reconciliation triggers
- Status condition management for observability
- Graceful degradation when metrics unavailable

### 2. Collector

**Location:** `internal/collector/`

Gathers metrics and cluster state information from multiple sources.

**Responsibilities:**
- Query Prometheus for vLLM inference metrics
- Collect pod and deployment information from Kubernetes API
- Cache metrics to reduce query frequency
- Enrich metrics with pod metadata (variant name, accelerator type, cost)

**Key Metrics Collected:**
- `vllm:kv_cache_usage_perc` - KV cache utilization (0.0-1.0)
- `vllm:num_requests_waiting` - Queue depth
- `vllm:gpu_cache_usage_perc` - GPU memory usage
- `vllm:time_to_first_token_seconds` - TTFT latency
- `vllm:time_per_output_token_seconds` - Inter-token latency

**Implementation Details:**
- Prometheus client with TLS support
- Background metrics fetching with caching
- Configurable query intervals
- Pod metadata enrichment via Kubernetes API

**Related Documentation:** [Prometheus Integration](../integrations/prometheus.md)

### 3. Saturation Analyzer

**Location:** `internal/saturation/analyzer.go`

Analyzes inference server saturation to detect capacity exhaustion and make scaling decisions.

**Responsibilities:**
- Identify saturated vs. non-saturated replicas
- Calculate spare capacity across variants
- Determine scale-up/scale-down recommendations
- Perform safety checks for scale-down operations
- Support multi-variant cost-aware decisions

**Algorithm:**
1. Classify replicas as saturated or non-saturated based on thresholds
2. Calculate average spare capacity across non-saturated replicas
3. Trigger scale-up if spare capacity < trigger threshold
4. Simulate load redistribution to verify scale-down safety
5. Generate per-variant scaling recommendations

**Key Thresholds (Configurable):**
- KV Cache Threshold: 80% (saturation point)
- Queue Length Threshold: 5 requests
- KV Spare Trigger: 10% (scale-up trigger)
- Queue Spare Trigger: 3 requests

**Related Documentation:** [Saturation Analyzer](../saturation-analyzer.md), [Saturation Scaling Configuration](../saturation-scaling-config.md)

### 4. Model Analyzer (Queue Theory)

**Location:** `pkg/analyzer/`

Uses queueing theory models (M/M/1/k, M/G/1) to predict performance and size servers.

**Responsibilities:**
- Analyze request latency and throughput
- Predict performance under different configurations
- Size servers to meet SLO requirements
- Estimate optimal batch sizes and queue lengths

**Models:**
- M/M/1/k: Markovian arrivals, service times, finite queue
- M/G/1: Markovian arrivals, general service times
- State-dependent service rates for batch processing

**Use Cases:**
- Offline capacity planning
- Performance prediction
- SLO validation
- Parameter estimation from benchmarks

**Related Documentation:** [Parameter Estimation Tutorial](../tutorials/parameter-estimation.md), [Modeling & Optimization](./modeling-optimization.md)

### 5. Optimizer

**Location:** `internal/optimizer/optimizer.go`, `pkg/solver/`

Makes global scaling decisions across models and variants to minimize cost while meeting SLOs.

**Responsibilities:**
- Optimize replica allocation across variants
- Consider cost constraints and preferences
- Respect capacity limits and SLO requirements
- Generate recommended replica counts

**Optimization Strategies:**
- Greedy allocation for cost minimization
- Constraint-based solver for complex scenarios
- Multi-objective optimization (cost, performance, utilization)

**Current Mode:** Unlimited capacity (each variant scaled independently)

**Future:** Limited capacity mode with cluster-wide resource constraints

**Related Documentation:** [Modeling & Optimization](./modeling-optimization.md)

### 6. Actuator

**Location:** `internal/actuator/actuator.go`

Executes scaling decisions by emitting metrics and updating Kubernetes resources.

**Responsibilities:**
- Emit custom metrics to Prometheus (desired replicas)
- Update VariantAutoscaling CR status
- Validate deployment targets
- Handle metric emission failures gracefully

**Metrics Emitted:**
- `inferno_desired_replicas` - Target replica count per variant
- `inferno_current_replicas` - Current replica count per variant
- `inferno_optimization_status` - Optimization result status

**Integration:**
- HPA/KEDA reads `inferno_desired_replicas` via Prometheus Adapter
- External scaler updates deployment replica count
- Controller reconciles on scaling events

**Related Documentation:** [HPA Integration](../integrations/hpa-integration.md), [KEDA Integration](../integrations/keda-integration.md)

### 7. Discovery

**Location:** `internal/discovery/`

Discovers available GPU accelerators in the cluster.

**Responsibilities:**
- Identify GPU types and models
- Query GPU operator for accelerator information
- Provide accelerator inventory to optimizer

**Supported Discovery Mechanisms:**
- NVIDIA GPU Operator (node labels)
- Manual configuration via ConfigMaps

**Current Implementation:** `K8sWithGpuOperator` discovers GPUs via node labels

**Future:** Support for AMD, Intel, and other accelerators

## Data Flow

### Reconciliation Cycle

```
1. Event Trigger (CR change, periodic sync, pod event)
   │
   ▼
2. Collector: Fetch metrics from Prometheus
   │
   ▼
3. Analyzer: Analyze saturation across replicas
   │
   ▼
4. Optimizer: Calculate optimal replica allocation (optional)
   │
   ▼
5. Actuator: Emit desired replica metrics to Prometheus
   │
   ▼
6. Controller: Update CR status with results
   │
   ▼
7. HPA/KEDA: Read metrics and scale deployment
   │
   ▼
8. Wait for reconciliation interval or next event
```

### Metrics Flow

```
vLLM Pods → Prometheus ← Collector → Analyzer
                ↑                        ↓
            Actuator ←── Optimizer ──────┘
                │
                ▼
        Prometheus Adapter
                │
                ▼
            HPA / KEDA
                │
                ▼
        Deployment Scaling
```

## Operating Modes

### 1. Saturation-Based Scaling (Primary Mode)

Uses real-time saturation metrics to make reactive scaling decisions.

**Enabled:** Default mode, always active
**Configuration:** Via `capacity-scaling-config` ConfigMap
**Decision Making:** Saturation Analyzer determines scale-up/scale-down
**Trigger:** Spare capacity falls below configured thresholds

**Advantages:**
- Fast response to load changes
- No offline profiling required
- Prevents capacity exhaustion
- Safe scale-down with simulation

### 2. Model-Based Optimization (Hybrid Mode)

Combines saturation analysis with queueing theory predictions.

**Enabled:** When model parameters configured
**Configuration:** Via per-model ConfigMaps with queue parameters
**Decision Making:** Hybrid arbitration between saturation and model-based targets
**Trigger:** Both saturation detection and SLO violations

**Advantages:**
- Predictive scaling before saturation
- SLO-aware optimization
- Cost-conscious allocation
- Performance modeling

### 3. Scale-from-Zero

Allows scaling deployments down to zero replicas when idle.

**Enabled:** Via HPA/KEDA configuration
**Configuration:** Set `minReplicas: 0` in HPA/KEDA
**Behavior:** WVA recommends 0 replicas when no traffic detected

**Related Documentation:** [Scale-from-Zero Guide](../user-guide/scale-from-zero.md)

## Configuration

### ConfigMaps

WVA uses several ConfigMaps for configuration:

1. **`capacity-scaling-config`** - Saturation thresholds per model
2. **`accelerator-unitcost`** - GPU cost data for optimization
3. **`serviceclass-config`** - Service class definitions and SLOs

**Location:** `deploy/` and `config/samples/`

### Environment Variables

Controller behavior can be customized via environment variables:

- `RECONCILIATION_INTERVAL` - Reconciliation period (default: 60s)
- `METRICS_CACHE_TTL` - Metrics cache duration (default: 30s)
- `PROMETHEUS_URL` - Prometheus endpoint
- `ENABLE_LEADER_ELECTION` - Leader election (default: true)

## Integration Points

### Prometheus

**Purpose:** Metrics source and sink
**Queries:** vLLM metrics for analysis
**Writes:** Custom metrics for HPA/KEDA

### HPA (Horizontal Pod Autoscaler)

**Purpose:** Execute scaling decisions
**Integration:** Reads `inferno_desired_replicas` metric via Prometheus Adapter
**Configuration:** Target metric value typically set to 1.0

### KEDA (Kubernetes Event-Driven Autoscaling)

**Purpose:** Alternative to HPA with more features
**Integration:** Prometheus scaler reads WVA metrics
**Advantages:** Scale-to-zero support, multiple triggers

### llm-d Infrastructure

**Purpose:** Model serving infrastructure
**Components:** Gateway, model registry, inference scheduler
**Integration:** WVA scales llm-d deployments

## Performance Characteristics

### Computational Complexity

- **Per-replica analysis:** O(N) where N = number of replicas
- **Variant aggregation:** O(V) where V = number of variants
- **Optimization:** O(V × A) where A = accelerator types

### Resource Usage

- **Memory:** ~50-100 MB base + metrics cache
- **CPU:** Low (<5% on typical clusters)
- **Network:** Prometheus queries every reconciliation cycle

### Scaling Performance

- **Detection latency:** 60s (default reconciliation interval)
- **Scale-up latency:** 60-120s (reconciliation + HPA + pod startup)
- **Scale-down latency:** 120-300s (HPA stabilization window)

## Observability

### Logging

Structured logging with multiple levels:

- **INFO:** Normal operations, scaling decisions
- **DEBUG:** Detailed analysis, metric values
- **ERROR:** Failures, retries, degradation

### Metrics (Emitted by WVA)

- `inferno_desired_replicas` - Recommended replica count
- `inferno_current_replicas` - Current replica count
- `inferno_optimization_status` - Success/failure indicator
- `controller_runtime_*` - Controller framework metrics

### Status Conditions

VariantAutoscaling resources track health via status conditions:

- `MetricsAvailable` - Prometheus metrics reachable
- `OptimizationSuccessful` - Last optimization succeeded
- `Ready` - Controller operating normally

**Related Documentation:** [Metrics Health Monitoring](../metrics-health-monitoring.md)

## Failure Modes and Resilience

### Prometheus Unavailable

**Behavior:** Graceful degradation - continue with last known state
**Status:** `MetricsAvailable` condition set to False
**Recovery:** Resume normal operation when Prometheus returns

### Invalid Configuration

**Behavior:** Use default values, log warnings
**Status:** `Ready` condition set to False with reason
**Recovery:** Fix configuration, controller auto-recovers

### Pod Metrics Missing

**Behavior:** Exclude pods with missing metrics from analysis
**Impact:** May result in conservative scaling decisions
**Mitigation:** Wait for pods to become ready and emit metrics

### HPA/KEDA Issues

**Behavior:** Continue emitting metrics, HPA/KEDA responsible for recovery
**Impact:** Scaling actions may be delayed
**Observability:** Check HPA/KEDA logs and events

## Security Considerations

### TLS for Prometheus

WVA supports TLS-encrypted connections to Prometheus:

- CA certificate mounted via ConfigMap
- mTLS client authentication (optional)
- Certificate validation enabled by default

### RBAC Permissions

Minimal RBAC permissions required:

- Read: Deployments, Pods, Nodes, ConfigMaps
- Write: VariantAutoscaling status
- Create: Events (for audit trail)

### Metrics Security

Custom metrics exposed via protected endpoint:

- Service mesh integration supported
- NetworkPolicy compatible
- Authentication via ServiceAccount token

## Limitations

1. **Prometheus Dependency:** Requires Prometheus for metrics
2. **vLLM-Specific Metrics:** Currently optimized for vLLM inference servers
3. **Single Model per CR:** One VariantAutoscaling resource per model deployment
4. **Unlimited Capacity Mode:** No cluster-wide resource constraints (current)
5. **Minimum Replicas:** Cannot scale below 1 replica without scale-to-zero

**Related Documentation:** [Architecture Limitations](./architecture-limitations.md)

## Future Enhancements

### Planned Features

- Multi-tenant resource quotas
- Predictive autoscaling with ML
- Custom metrics adapter (eliminate Prometheus Adapter dependency)
- Support for additional inference frameworks (TensorRT-LLM, TGI)
- Advanced cost optimization with spot instances

### Research Areas

- Federated learning for cross-cluster optimization
- Request routing integration with inference scheduler
- Dynamic batch size tuning
- Multi-objective optimization (cost, latency, throughput)

## Related Documentation

- [Saturation Analyzer](../saturation-analyzer.md) - Deep dive into saturation analysis
- [Modeling & Optimization](./modeling-optimization.md) - Queue theory and optimization
- [Configuration Guide](../user-guide/configuration.md) - Configuration options
- [Developer Guide](../developer-guide/development.md) - Development setup
- [Testing Guide](../developer-guide/testing.md) - Testing strategies

## References

- [Kubernetes Controllers](https://kubernetes.io/docs/concepts/architecture/controller/)
- [Prometheus Operator](https://prometheus-operator.dev/)
- [HPA Documentation](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)
- [KEDA Documentation](https://keda.sh/)
- [Queueing Theory Fundamentals](https://en.wikipedia.org/wiki/Queueing_theory)
