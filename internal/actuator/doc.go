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
Package actuator provides metrics emission for external autoscalers.

# Overview

The actuator package is responsible for emitting the inferno_desired_replicas
metric to Prometheus, which is consumed by HPA or KEDA for scaling decisions.
It acts as the bridge between WVA's scaling analysis and Kubernetes autoscaling.

# Key Responsibilities

Metrics Emission:
  - Emit inferno_desired_replicas metric with variant-specific labels
  - Expose metrics endpoint for Prometheus scraping
  - Update metrics atomically to prevent inconsistent states

Status Tracking:
  - Track optimization status and metadata
  - Maintain per-variant allocation information
  - Provide observability into scaling decisions

# Metrics

The primary metric emitted:

	inferno_desired_replicas{
		variant_name="vllm-deployment-decode",
		model_id="meta/llama-3.1-8b",
		accelerator="H100",
		namespace="llm-inference"
	} = 5

This metric indicates that WVA has determined the variant should have 5 replicas.

# Integration Flow

	Controller
	    │
	    ├──▶ Analysis & Optimization
	    │
	    └──▶ Actuator.EmitMetrics(desired_replicas)
	              │
	              ▼
	        Prometheus (scrape)
	              │
	              ▼
	      Prometheus Adapter
	              │
	              ▼
	          HPA/KEDA (read external metric)
	              │
	              ▼
	         Scale Deployment

# Usage Example

	actuator := actuator.NewActuator()

	err := actuator.EmitMetrics(ctx, variantName, modelID, accelerator, desiredReplicas)
	if err != nil {
		log.Error(err, "Failed to emit metrics")
	}

# Metrics Endpoint

The actuator exposes a Prometheus metrics endpoint:
  - Path: /metrics
  - Port: Configured via controller (default: 8443 for secure metrics)
  - Format: Prometheus text exposition format

# Labels

The inferno_desired_replicas metric includes these labels:
  - variant_name: Name of the variant (typically deployment name)
  - model_id: HuggingFace model identifier
  - accelerator: GPU type (A100, H100, L40S, etc.)
  - namespace: Kubernetes namespace

These labels allow:
  - Multiple variants to coexist
  - HPA/KEDA to target specific variants
  - Cost and capacity tracking per accelerator type

# HPA Integration

HPA configuration example:

	apiVersion: autoscaling/v2
	kind: HorizontalPodAutoscaler
	metadata:
	  name: vllm-deployment-hpa
	spec:
	  scaleTargetRef:
	    apiVersion: apps/v1
	    kind: Deployment
	    name: vllm-deployment-decode
	  maxReplicas: 10
	  metrics:
	  - type: External
	    external:
	      metric:
	        name: inferno_desired_replicas
	        selector:
	          matchLabels:
	            variant_name: vllm-deployment-decode
	      target:
	        type: AverageValue
	        averageValue: "1"

The HPA compares current replicas against the metric value and scales accordingly.

# KEDA Integration

KEDA ScaledObject example:

	apiVersion: keda.sh/v1alpha1
	kind: ScaledObject
	metadata:
	  name: vllm-deployment-scaler
	spec:
	  scaleTargetRef:
	    name: vllm-deployment-decode
	  maxReplicaCount: 10
	  triggers:
	  - type: prometheus
	    metadata:
	      serverAddress: http://prometheus:9090
	      query: |
	        inferno_desired_replicas{variant_name="vllm-deployment-decode"}
	      threshold: "1"

# Status Management

The actuator tracks optimization status:
  - Last optimization timestamp
  - Last successful metrics emission
  - Optimization metadata (analysis results, warnings)
  - Error conditions

This information is used by the controller to update VariantAutoscaling status.

# Error Handling

The actuator handles:
  - Metrics server unavailable: Log warning, continue
  - Invalid metric values: Validate before emission
  - Concurrent updates: Thread-safe metric updates

# Observability

The actuator provides:
  - Standard Prometheus metrics (scrape duration, error count)
  - Controller-runtime metrics integration
  - Structured logging for debugging

# Security

Metrics endpoint security:
  - TLS support for secure metrics
  - RBAC for metrics reader role
  - No sensitive data in metric labels

# Performance

Metrics emission performance:
  - Low latency: Sub-millisecond metric updates
  - No external dependencies for emission
  - Efficient label cardinality

# Testing

The actuator is tested:
  - Unit tests: Metric emission correctness
  - Integration tests: HPA/KEDA reading metrics
  - E2E tests: End-to-end scaling validation

# Integration Points

Integrates with:
  - internal/controller: Called during reconciliation
  - Prometheus: Metrics scraping
  - Prometheus Adapter: External metrics API
  - HPA/KEDA: Autoscaling consumers

# See Also

For HPA integration details, see docs/integrations/hpa-integration.md
For KEDA integration details, see docs/integrations/keda-integration.md
For architecture overview, see docs/architecture.md
*/
package actuator
