# Architecture Overview

This document provides a high-level overview of the Workload-Variant-Autoscaler (WVA) architecture, components, and design principles.

## System Architecture

WVA is a Kubernetes controller that enables intelligent autoscaling for LLM inference workloads. It operates as part of a larger ecosystem:

```
┌─────────────────────────────────────────────────────────────────┐
│                        User/Admin                                │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                  ┌─────────▼──────────┐
                  │ VariantAutoscaling │ (Custom Resource)
                  │        CR          │
                  └─────────┬──────────┘
                            │
┌───────────────────────────▼───────────────────────────────────┐
│                     WVA Controller                            │
│  ┌───────────┐  ┌────────────┐  ┌─────────┐  ┌────────────┐ │
│  │Reconciler │─▶│ Collector  │─▶│Analyzer │─▶│  Actuator  │ │
│  └───────────┘  └────────────┘  └─────────┘  └────────────┘ │
│       │              │               │              │          │
└───────┼──────────────┼───────────────┼──────────────┼────────┘
        │              │               │              │
        │         ┌────▼───┐      ┌────▼────┐    ┌───▼────┐
        │         │Prometheus│     │Capacity │    │Prometheus│
        │         │  (Read)  │     │  Model  │    │ (Write) │
        │         └──────────┘     └─────────┘    └────────┘
        │
        │         ┌────────────────────────────────────┐
        └────────▶│ HPA/KEDA (External Autoscaler)    │
                  └────────┬───────────────────────────┘
                           │
                  ┌────────▼──────────┐
                  │   Deployment      │
                  │  (Model Server)   │
                  └───────────────────┘
```

## Core Components

### 1. Reconciler

**Purpose**: Kubernetes controller that manages the lifecycle of VariantAutoscaling resources.

**Responsibilities**:
- Watches for VariantAutoscaling CR creation, updates, and deletion
- Validates CR specifications
- Coordinates data collection, analysis, and actuation
- Updates CR status with current state
- Handles error conditions and retries

**Implementation**: `internal/controller/variantautoscaling_controller.go`

**Key Features**:
- Event-driven reconciliation
- Predicate-based filtering for efficiency
- Leader election support for high availability
- Graceful handling of deployment lifecycle

### 2. Collector

**Purpose**: Gathers metrics and cluster state information needed for scaling decisions.

**Responsibilities**:
- Queries Prometheus for vLLM server metrics
- Retrieves deployment status from Kubernetes API
- Caches metrics to reduce query load
- Tracks metric freshness and validity

**Implementation**: `internal/collector/`

**Collected Metrics**:
- **KV Cache Utilization**: GPU memory usage for key-value cache
- **Queue Depth**: Number of requests waiting in queue
- **Request Rate**: Incoming request rate per pod
- **GPU Memory**: Total and available GPU memory
- **Replica Count**: Current number of running replicas

**Prometheus Queries**:
```promql
# KV cache usage percentage
avg by (pod) (vllm_cache_usage{namespace="<ns>",deployment="<name>"})

# Request queue depth
avg by (pod) (vllm_queue_depth{namespace="<ns>",deployment="<name>"})

# Request rate
rate(vllm_requests_total{namespace="<ns>",deployment="<name>"}[5m])
```

### 3. Saturation Analyzer

**Purpose**: Analyzes collected metrics to determine current saturation level and capacity.

**Responsibilities**:
- Evaluates server saturation based on KV cache and queue depth
- Identifies replicas with slack capacity
- Determines if scaling is needed
- Provides recommendations for replica count

**Implementation**: `internal/saturation/analyzer.go`

**Analysis Algorithm**:
1. Calculate per-replica saturation score
2. Identify overloaded replicas (saturation > threshold)
3. Identify underutilized replicas (saturation < threshold)
4. Compute total system capacity and slack
5. Recommend optimal replica count

**Saturation Formula**:
```
saturation = max(kv_cache_utilization, queue_depth_factor)

where:
  queue_depth_factor = min(queue_depth / max_queue_depth, 1.0)
```

### 4. Capacity Model Engine

**Purpose**: Implements the saturation-based scaling model.

**Responsibilities**:
- Applies scaling thresholds and policies
- Considers min/max replica constraints
- Implements scale-up and scale-down logic
- Provides gradual scaling behavior

**Implementation**: `internal/engines/saturation/engine.go`

**Scaling Logic**:
```
if avg_saturation > scale_up_threshold:
    desired_replicas = current_replicas + scale_up_increment
elif avg_saturation < scale_down_threshold:
    desired_replicas = current_replicas - scale_down_increment
else:
    desired_replicas = current_replicas  # Stay in stability zone

desired_replicas = clamp(desired_replicas, min_replicas, max_replicas)
```

### 5. Actuator

**Purpose**: Emits computed metrics and updates Kubernetes resources.

**Responsibilities**:
- Writes desired replica count to Prometheus as custom metric
- Updates VariantAutoscaling CR status
- Records events for observability
- Ensures metric freshness

**Implementation**: `internal/actuator/actuator.go`

**Emitted Metrics**:
```promql
# Desired replica count for HPA/KEDA
wva_desired_replicas{variant="<name>",namespace="<ns>"} <value>

# Current saturation level
wva_saturation_level{variant="<name>",namespace="<ns>"} <value>

# Scaling decision timestamp
wva_last_scaling_decision_timestamp{variant="<name>"} <timestamp>
```

## Data Flow

### Reconciliation Loop

```
1. Event triggers reconciliation
   ├─ VariantAutoscaling CR created/updated
   ├─ Periodic resync (default: 60s)
   └─ Deployment changes

2. Collector phase
   ├─ Query Prometheus for vLLM metrics
   ├─ Retrieve deployment status
   ├─ Check cache for recent data
   └─ Validate metric completeness

3. Analysis phase
   ├─ Calculate per-replica saturation
   ├─ Compute system-wide capacity
   ├─ Determine if scaling needed
   └─ Generate scaling recommendation

4. Actuation phase
   ├─ Emit desired_replicas metric
   ├─ Update CR status
   ├─ Record events
   └─ Update conditions

5. External scaling (HPA/KEDA)
   ├─ Read wva_desired_replicas metric
   ├─ Compare with current replicas
   ├─ Apply scaling behavior policies
   └─ Scale deployment
```

### Metric Flow

```
vLLM Pod ─────┐
vLLM Pod ─────┼─▶ Prometheus ─▶ WVA Collector ─▶ Analyzer
vLLM Pod ─────┘                                       │
                                                      ▼
                                         ┌────────────────────┐
                                         │ Capacity Model     │
                                         └─────────┬──────────┘
                                                   │
                     ┌─────────────────────────────┘
                     ▼
              WVA Actuator ─▶ Prometheus ─▶ HPA/KEDA ─▶ Deployment
```

## Design Principles

### 1. Separation of Concerns

WVA **computes** optimal scaling decisions but **does not directly scale** deployments. This design:
- Leverages battle-tested HPA/KEDA for actual scaling operations
- Allows gradual scaling with stabilization windows
- Enables policy-based scaling behavior
- Maintains compatibility with existing autoscaling tools

### 2. Metrics-Driven Architecture

All decisions are based on observable metrics:
- **No guessing**: Scales based on actual server saturation
- **Transparent**: All data sources and calculations are visible
- **Debuggable**: Metrics can be queried and visualized in Prometheus
- **Auditable**: Decision history is preserved

### 3. Graceful Degradation

WVA handles failures gracefully:
- **Missing metrics**: Uses cached data when available
- **Prometheus unavailable**: Skips scaling decisions but maintains status
- **Deployment deleted**: Updates CR status, resumes when deployment returns
- **Invalid configuration**: Reports errors in status conditions

### 4. Eventual Consistency

WVA operates in an eventually consistent manner:
- Reconciliation is periodic, not real-time
- HPA/KEDA adds additional latency for stability
- Total response time: 2-5 minutes typical

This is acceptable for LLM inference workloads where:
- Traffic patterns change gradually
- Pod startup time is measured in minutes
- Stability is preferred over immediate response

## Configuration & Customization

### Controller Configuration

Configuration is provided via ConfigMaps:

- **Main Configuration** (`wva-configmap`): Controller behavior
- **Saturation Scaling** (`saturation-scaling-config`): Scaling thresholds and policies
- **Accelerator Costs** (`accelerator-costs`): Cost per accelerator type
- **Service Classes** (`service-class-config`): SLO definitions (future use)

### Per-Variant Configuration

Each VariantAutoscaling CR can specify:
- **ModelID**: Identifier for the model variant
- **ScaleTargetRef**: Target deployment to scale
- **ModelProfile**: Accelerator types and resource requirements
- **VariantCost**: Cost metric for optimization

## Integration Points

### Prometheus

**Integration Type**: Bidirectional
- **Read**: Collects vLLM metrics for analysis
- **Write**: Emits computed metrics for HPA/KEDA

**Requirements**:
- Prometheus Operator or standalone Prometheus
- ServiceMonitor for vLLM pods
- Prometheus Adapter for external metrics API

### HPA (Horizontal Pod Autoscaler)

**Integration Type**: Unidirectional (WVA → HPA)
- WVA emits `wva_desired_replicas` metric
- HPA reads metric via Prometheus Adapter
- HPA scales deployment based on target value

**Configuration Example**:
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: model-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: vllm-model
  metrics:
  - type: External
    external:
      metric:
        name: wva_desired_replicas
        selector:
          matchLabels:
            variant: model-name
      target:
        type: Value
        value: "1"
```

### KEDA (Kubernetes Event Driven Autoscaling)

**Integration Type**: Unidirectional (WVA → KEDA)
- Similar to HPA but with more flexible triggers
- Supports scale-to-zero (with limitations)
- Advanced scaling behaviors

See [KEDA Integration](../integrations/keda-integration.md) for details.

## Scalability & Performance

### Controller Scalability

- **Concurrent reconciliations**: Limited by `--max-concurrent-reconciles` flag
- **Recommended**: 1 controller per cluster, multiple controllers for isolation
- **Resources**: Minimal CPU/memory footprint (< 100m CPU, < 128Mi memory typical)

### Prometheus Query Load

- **Query frequency**: Once per reconciliation cycle (60s default)
- **Query complexity**: Bounded by number of replicas and time range
- **Optimization**: Result caching reduces redundant queries

### Multi-Tenancy

Multiple approaches for multi-tenant deployments:
1. **Single controller, namespace isolation**: Default approach
2. **Multiple controllers with label selectors**: Enterprise isolation
3. **Controller per namespace**: Maximum isolation

See [Multi-Controller Isolation](../user-guide/multi-controller-isolation.md).

## Security Considerations

### RBAC Permissions

WVA requires:
- **Read**: Deployments, ReplicaSets, Pods
- **Read/Write**: VariantAutoscaling CRs
- **Read**: ConfigMaps (for configuration)
- **Read**: Secrets (for Prometheus TLS certificates)

### Prometheus Authentication

Supports:
- **Bearer token authentication**: Via ServiceAccount token
- **TLS with CA certificate**: For secure connections
- **mTLS**: Mutual TLS authentication

### Metrics Exposure

Controller metrics are exposed via:
- **Authenticated endpoint**: Requires token for access
- **TLS encryption**: HTTPS only
- **RBAC protected**: ClusterRole controls access

## Observability

### Logging

Structured logging with configurable levels:
- **Info**: Normal operations, scaling decisions
- **Debug**: Detailed metric collection, calculations
- **Error**: Failures, retries, validation errors

### Metrics

Controller exposes metrics:
- **Reconciliation duration**: Histogram of reconcile time
- **Reconciliation errors**: Counter of failures
- **Queue depth**: Current work queue size
- **API call duration**: Latency of Kubernetes API calls

### Events

Kubernetes events are emitted for:
- Scaling decisions
- Configuration errors
- Metric collection failures
- Status updates

## Limitations & Constraints

See [Architecture Limitations](architecture-limitations.md) for detailed information on:
- Model architecture constraints (HSSM, MoE not supported)
- Metric requirements
- Scaling boundaries
- Performance considerations

## Related Documentation

- [Design: Modeling & Optimization](modeling-optimization.md)
- [Design: Controller Behavior](controller-behavior.md)
- [Saturation Scaling Algorithm](../saturation-scaling-config.md)
- [Developer Guide](../developer-guide/development.md)
- [API Reference](../user-guide/crd-reference.md)

---

For questions or clarifications about the architecture, open a discussion or issue on GitHub.
