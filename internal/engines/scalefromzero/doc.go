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
Package scalefromzero implements scale-from-zero optimization for idle workloads.

# Overview

The scale-from-zero engine handles workloads that have been scaled down to zero
replicas due to inactivity. It monitors for incoming traffic and triggers
scale-up when requests arrive.

# Key Features

  - Detects zero-replica deployments
  - Monitors for incoming traffic signals
  - Triggers rapid scale-up when traffic resumes
  - Coordinates with HPA/KEDA for replica management
  - Prevents premature scale-down with cooldown periods

# Architecture

The scale-from-zero engine operates independently or alongside other engines:

	1. Identifies VariantAutoscaling resources with zero replicas
	2. Monitors traffic indicators (queue metrics, request rates)
	3. Emits scale-up signal when traffic detected
	4. Hands off to saturation/model engines once replicas are running

# Use Cases

Scale-from-zero is useful for:

  - Development/test environments with intermittent usage
  - Cost optimization for infrequently used models
  - Batch processing workloads with predictable patterns
  - Multi-tenant systems with sporadic model access

# Traffic Detection

The engine monitors multiple signals:

  - Request queue depth in load balancer/gateway
  - Prometheus metrics from ingress components
  - Custom application metrics

When traffic is detected, the engine immediately recommends scaling to
minimum viable replicas (typically 1-2).

# Integration

Scale-from-zero integrates with:

  - internal/collector: Monitors traffic metrics
  - internal/actuator: Emits scale-up signals
  - HPA/KEDA: Executes actual scaling operations
  - Other engines: Hands off once replicas are running

# Configuration

Scale-from-zero behavior is controlled by deployment configuration:

	apiVersion: llmd.ai/v1alpha1
	kind: VariantAutoscaling
	spec:
	  modelID: "meta/llama-3.1-8b"
	  minReplicas: 0  # Enable scale-to-zero

When minReplicas > 0, scale-from-zero engine is not activated.

# Cooldown Periods

To prevent flapping, the engine implements cooldown:

  - Scale-up cooldown: Minimum time before scaling from zero (default: 0s)
  - Scale-down cooldown: Minimum idle time before scaling to zero (default: 300s)

These are typically configured in HPA/KEDA rather than the WVA engine itself.

# Cold Start Considerations

Scaling from zero incurs cold start latency:

  - Pod startup time: 30-60 seconds
  - Model loading time: 30-120 seconds (depends on model size)
  - Total cold start: 60-180 seconds

For latency-sensitive workloads, keep minReplicas >= 1 to maintain warm capacity.

# Usage

The scale-from-zero engine is automatically activated when:

	1. VariantAutoscaling exists with minReplicas: 0
	2. Current replica count is 0
	3. No other engine is actively managing the workload

No explicit configuration is required beyond setting minReplicas.

# Execution Strategy

The engine uses polling with short intervals when zero replicas detected:

	pollInterval = 5 * time.Second  // Check for traffic frequently

Once traffic is detected and scale-up initiated, the engine transitions
to monitoring mode until replicas are running.

# Thread Safety

The scale-from-zero engine is safe for single-threaded usage per instance.
The polling executor manages execution scheduling.

# Performance

Traffic detection latency: <10 seconds
Scale-up signal emission: <1 second
Total time to first request served: 60-180 seconds (includes cold start)

# Limitations

  - Requires monitoring infrastructure to detect incoming traffic
  - Cold start latency may violate SLOs for first requests
  - Not suitable for latency-critical workloads
  - May require custom metrics for traffic detection

# Best Practices

For scale-from-zero deployments:

  1. Configure appropriate cooldown periods to prevent flapping
  2. Use health checks to ensure models fully loaded before serving
  3. Consider keeping warm capacity (minReplicas: 1) for critical services
  4. Monitor cold start metrics to understand user impact
  5. Implement queue-based buffering to handle requests during scale-up

# Status

The scale-from-zero engine is experimental and not recommended for production
workloads with strict SLO requirements. Use with caution and thorough testing.
*/
package scalefromzero
