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
Package actuator provides metrics emission for scaling recommendations.

# Overview

The actuator package is responsible for emitting WVA's scaling recommendations
as Prometheus metrics that can be consumed by external autoscalers (HPA, KEDA).
It serves as the output layer that translates optimization results into
actionable metrics.

# Architecture

The actuator implements a simple emit-and-update pattern:

	Optimization Results → Actuator → Prometheus Metrics → HPA/KEDA

The actuator does not perform actual scaling operations; it only emits
metrics that external autoscalers use to make scaling decisions.

# Metrics Emitted

Primary Scaling Metric:

	inferno_desired_replicas{
	  namespace="llm-inference",
	  deployment="llama-8b-vllm",
	  model_id="meta/llama-3.1-8b",
	  variant_name="llama-8b-a100"
	} = 5

This metric indicates the recommended number of replicas for a deployment.

Optimization Metadata:

	inferno_optimization_cost{
	  namespace="...",
	  deployment="...",
	  model_id="..."
	} = 150.0

	inferno_allocation_accelerator{
	  namespace="...",
	  deployment="...",
	  model_id="...",
	  accelerator="A100"
	} = 1

# Integration with Autoscalers

HPA Integration:

	apiVersion: autoscaling/v2
	kind: HorizontalPodAutoscaler
	metadata:
	  name: llama-8b-hpa
	spec:
	  scaleTargetRef:
	    apiVersion: apps/v1
	    kind: Deployment
	    name: llama-8b-vllm
	  metrics:
	    - type: External
	      external:
	        metric:
	          name: inferno_desired_replicas
	          selector:
	            matchLabels:
	              deployment: llama-8b-vllm
	        target:
	          type: Value
	          value: "1"

KEDA Integration:

	apiVersion: keda.sh/v1alpha1
	kind: ScaledObject
	metadata:
	  name: llama-8b-scaler
	spec:
	  scaleTargetRef:
	    name: llama-8b-vllm
	  triggers:
	    - type: prometheus
	      metadata:
	        serverAddress: http://prometheus:9090
	        query: inferno_desired_replicas{deployment="llama-8b-vllm"}
	        threshold: "1"

# Metric Lifecycle

The actuator manages metric lifecycle:

Creation:

Metrics are created when VariantAutoscaling resources are first processed.

Updates:

Metrics are updated on each reconciliation with new recommendations.

Cleanup:

Metrics are NOT automatically removed when resources are deleted. HPA/KEDA
handle stale metrics through their own timeout mechanisms.

# Implementation

The actuator uses the Prometheus client library:

	import "github.com/prometheus/client_golang/prometheus"

	desiredReplicasGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "inferno_desired_replicas",
			Help: "Desired number of replicas for deployment",
		},
		[]string{"namespace", "deployment", "model_id", "variant_name"},
	)

# Thread Safety

The actuator is thread-safe. Prometheus client library handles concurrent
metric updates internally with appropriate synchronization.

# Error Handling

Metrics emission errors are logged but not fatal:

  - Duplicate metric registration: Logged as warning
  - Invalid label values: Sanitized automatically
  - Prometheus collector errors: Logged and retried

# Configuration

The actuator requires minimal configuration:

  - Prometheus metrics port: Configured in controller (default: 8080)
  - Metric prefix: Hardcoded as "inferno_"
  - Label schema: Defined by integration contracts

# Usage

The actuator is used by the controller and engines:

	import "github.com/llm-d-incubation/workload-variant-autoscaler/internal/actuator"

	actuator := actuator.New()
	actuator.EmitDesiredReplicas(
		namespace,
		deploymentName,
		modelID,
		variantName,
		desiredReplicas,
	)

# Metrics Endpoint

Metrics are exposed on the controller's metrics endpoint:

	GET http://controller:8080/metrics

This endpoint is typically scraped by Prometheus every 10-30 seconds.

# Best Practices

For metrics emission:

  1. Emit metrics only after successful optimization
  2. Use consistent label values across reconciliations
  3. Avoid metric churn (frequent creation/deletion)
  4. Log metrics emission for debugging
  5. Monitor scrape failures in Prometheus

# Integration Testing

The actuator can be tested in isolation:

	import "github.com/prometheus/client_golang/prometheus/testutil"

	actuator.EmitDesiredReplicas("test", "deploy", "model", "variant", 5)
	value := testutil.ToFloat64(desiredReplicasGauge.WithLabelValues(...))
	assert.Equal(t, 5.0, value)

# Performance

Metric emission is extremely fast:
  - Single metric update: <1µs
  - Batch of 10 metrics: <10µs

Metrics emission does not impact reconciliation performance.

# Observability

The actuator itself does not emit observability metrics. Monitor the
metrics endpoint scrape success rate in Prometheus to ensure metrics
are being collected properly.
*/
package actuator
