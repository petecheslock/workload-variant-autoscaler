/*
Copyright 2025 The llm-d Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
Package controller implements the Kubernetes controller for VariantAutoscaling resources.

# Overview

This package provides the reconciliation logic for the VariantAutoscaling custom
resource. It implements the controller-runtime Reconciler interface and coordinates
all WVA components to provide intelligent autoscaling for inference workloads.

# Architecture

The controller follows the Kubernetes operator pattern:

	Watch → Reconcile → Update Status → Emit Metrics

Key responsibilities:

  1. Watch VariantAutoscaling resources and related objects (Deployments, ConfigMaps)
  2. Coordinate metrics collection, analysis, and optimization
  3. Emit scaling recommendations via Prometheus metrics
  4. Update VariantAutoscaling status with current allocations
  5. Handle error conditions and degraded states

# Reconciliation Loop

The reconciliation process:

	1. Validate VariantAutoscaling resource and target deployment
	2. Select appropriate engine (saturation, model, hybrid)
	3. Collect metrics from inference servers
	4. Run analysis/optimization
	5. Emit recommendations to Prometheus
	6. Update VariantAutoscaling status
	7. Record events and update conditions

# Engine Selection

The controller selects engines based on configuration:

CAPACITY-ONLY Mode (default):

	env:
	  - name: EXPERIMENTAL_PROACTIVE_MODEL
	    value: "false"

Uses saturation engine only for fast, reactive scaling.

HYBRID Mode (experimental):

	env:
	  - name: EXPERIMENTAL_PROACTIVE_MODEL
	    value: "true"

Uses both saturation and model engines with arbitration logic.

# Metrics Emission

The controller emits Prometheus metrics for external autoscalers:

	inferno_desired_replicas{
	  namespace="...",
	  deployment="...",
	  model_id="..."
	} = 5

HPA or KEDA can scale deployments based on these metrics.

# Status Management

The controller updates VariantAutoscaling status with:

	status:
	  currentAllocation:
	    replicas: 3
	    acceleratorType: "A100"
	  desiredAllocation:
	    replicas: 5
	    acceleratorType: "A100"
	  conditions:
	    - type: MetricsAvailable
	      status: "True"
	    - type: OptimizationSucceeded
	      status: "True"

# Predicates

The controller uses predicates to filter events:

  - Generation changes on VariantAutoscaling
  - Spec or replicas changes on Deployments
  - Data changes on ConfigMaps

This prevents unnecessary reconciliations and improves performance.

# Error Handling

The controller handles errors gracefully:

Transient Errors:

Retries with exponential backoff:

	- Metrics collection failures
	- Temporary API errors
	- Prometheus connection issues

Permanent Errors:

Records in status and events:

	- Invalid configurations
	- Deployment not found
	- Unsupported model profiles

Degraded Mode:

Continues operation with limitations:

	- Falls back to last known good allocation
	- Uses default configurations when ConfigMaps unavailable
	- Logs warnings for missing optional data

# Integration

The controller integrates with:

  - internal/engines: Optimization engines
  - internal/collector: Metrics collection
  - internal/actuator: Metrics emission
  - internal/interfaces: Component contracts
  - api/v1alpha1: CRD definitions

# Configuration

Controller configuration is managed via:

Environment Variables:
  - EXPERIMENTAL_PROACTIVE_MODEL: Engine selection
  - PROMETHEUS_URL: Prometheus connection
  - METRICS_INTERVAL: Reconciliation interval

ConfigMaps:
  - capacity-scaling-config: Saturation thresholds
  - accelerator-unitcost: GPU pricing
  - serviceclass: SLO targets

# Watches

The controller watches:

  - VariantAutoscaling: Primary resource
  - Deployments: Target workloads
  - ConfigMaps: Configuration updates

ConfigMap changes trigger immediate reconciliation for affected resources.

# Leader Election

The controller supports leader election for high availability:

	mgr.LeaderElection = true
	mgr.LeaderElectionID = "workload-variant-autoscaler"

Only the leader performs reconciliation; followers remain on standby.

# Metrics

The controller exposes operational metrics:

  - Reconciliation duration (histogram)
  - Reconciliation errors (counter)
  - Active VariantAutoscaling resources (gauge)
  - Optimization successes/failures (counter)

# Thread Safety

The controller is safe for concurrent reconciliation of different resources.
Controller-runtime ensures single-threaded reconciliation per resource instance.

# Performance

Typical reconciliation time:
  - Saturation mode: 2-5 seconds
  - Hybrid mode: 5-10 seconds

Maximum concurrent reconciliations: Configurable (default: 1)

# Testing

The controller supports envtest for integration testing:

	import "sigs.k8s.io/controller-runtime/pkg/envtest"

	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{"../../config/crd/bases"},
	}

See internal/controller/suite_test.go for examples.

# Best Practices

For controller development:

  1. Keep reconciliation logic idempotent
  2. Use predicates to minimize unnecessary reconciliations
  3. Update status before returning to record progress
  4. Use exponential backoff for retries
  5. Record events for important state changes
  6. Validate inputs early to fail fast
  7. Handle partial failures gracefully
*/
package controller
