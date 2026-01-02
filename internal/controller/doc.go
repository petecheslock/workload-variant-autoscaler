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

This package provides the main reconciliation loop that watches VariantAutoscaling
custom resources and coordinates the autoscaling pipeline: metrics collection,
saturation analysis, optimization, and actuator emission.

# Key Components

VariantAutoscalingReconciler:
  - Main controller implementing controller-runtime Reconciler interface
  - Watches VariantAutoscaling CRs, Deployments, and ServiceMonitors
  - Coordinates the end-to-end autoscaling workflow
  - Updates VariantAutoscaling status with current and desired allocations
  - Emits Kubernetes events for important state changes

# Reconciliation Flow

The reconciliation process follows these steps:

1. **Fetch VariantAutoscaling CR**: Retrieve the resource from the API server

2. **Validate Spec**: Ensure modelID and scaleTargetRef are valid

3. **Collect Metrics**: Query Prometheus for saturation metrics
   - KV cache usage percentage
   - Queue length
   - Pod readiness and phase

4. **Analyze Saturation**: Determine scaling decisions
   - Classify saturated vs. non-saturated replicas
   - Calculate spare capacity
   - Determine scale-up/down needs

5. **Execute Engine**: Run the appropriate scaling engine
   - Saturation engine (current default)
   - Model engine (experimental)
   - Scale-from-zero engine (cold start)

6. **Emit Metrics**: Publish desired_replicas metric
   - Actuator emits to Prometheus
   - HPA/KEDA reads via Prometheus Adapter

7. **Update Status**: Write results to VariantAutoscaling status
   - currentReplicas: Current deployment replica count
   - desiredReplicas: Calculated target replica count
   - conditions: MetricsAvailable, OptimizationComplete, etc.
   - optimization metadata: Timestamps, analysis results

# Watched Resources

The controller watches:
  - VariantAutoscaling: Primary resource
  - Deployment: Scale target status changes
  - ServiceMonitor: Metrics collection configuration changes

# Predicates

Event filtering is implemented in predicates.go:
  - Update predicates: Filter out no-op updates
  - Generation predicates: Only react to spec changes
  - Deletion predicates: Handle cleanup

# Configuration

The controller is configured via:
  - Environment variables (PROMETHEUS_URL, LOG_LEVEL, etc.)
  - ConfigMaps (service-classes, model-accelerator-data)
  - VariantAutoscaling CR spec

# Error Handling

The controller implements exponential backoff retry:
  - Transient errors: Requeue with exponential backoff
  - Permanent errors: Don't requeue, set condition to False
  - Metrics unavailable: Retry with warning condition

# Events

Kubernetes events emitted:
  - Normal/OptimizationComplete: Successful scaling decision
  - Warning/MetricsUnavailable: Cannot collect metrics
  - Warning/ScaleDecisionFailed: Analysis error
  - Normal/DeploymentNotFound: Target deployment missing

# Usage Example

Setting up the controller:

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
	})

	reconciler := &controller.VariantAutoscalingReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("variantautoscaling-controller"),
	}

	err = reconciler.SetupWithManager(mgr)
	if err != nil {
		log.Fatal(err)
	}

	err = mgr.Start(ctrl.SetupSignalHandler())

# Status Conditions

The controller maintains these conditions:
  - MetricsAvailable: True if metrics successfully collected
  - OptimizationComplete: True if analysis succeeded
  - Ready: Overall readiness status

# RBAC

Required permissions:
  - variantautoscalings: get, list, watch, update, patch, updateStatus
  - deployments: get, list, watch
  - pods: get, list, watch
  - servicemonitors: get, list, watch
  - events: create, patch

# Scalability

Single active controller with leader election:
  - Multiple replicas supported
  - Only leader performs reconciliation
  - Followers stand by for failover

# Integration Points

Integrates with:
  - internal/collector: Metrics collection
  - internal/saturation: Saturation analysis
  - internal/engines: Scaling engines
  - internal/actuator: Metrics emission
  - internal/optimizer: Optimization coordination

# See Also

For architecture overview, see docs/architecture.md
For development guide, see docs/developer-guide/development.md
For CRD reference, see docs/user-guide/crd-reference.md
*/
package controller
