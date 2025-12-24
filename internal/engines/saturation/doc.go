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
Package saturation implements the saturation-based optimization engine for WVA.

# Overview

The saturation engine is the default and recommended optimization engine for
production deployments. It provides fast, reactive scaling decisions based on
real-time capacity metrics from inference servers.

# Key Features

  - Reactive scaling based on KV cache utilization and queue depth
  - Per-variant cost-aware scaling decisions
  - Spare capacity analysis for proactive scale-up
  - Scale-down safety validation via worst-case simulation
  - Sub-30 second response time
  - No model training or offline profiling required

# Architecture

The engine operates in two modes:

1. CAPACITY-ONLY mode (default, recommended for production):
   - Pure saturation-based scaling
   - Uses SaturationAnalyzer for capacity decisions
   - Fast response time (<30s)

2. HYBRID mode (experimental):
   - Combines saturation analyzer with model-based optimizer
   - Arbitrates between capacity and model-based targets
   - Capacity safety always overrides model predictions

# Scaling Logic

Scale-up triggers when:
  - Average spare KV capacity < kvSpareTrigger (default: 10%)
  - Average spare queue capacity < queueSpareTrigger (default: 2 requests)

Scale-down allowed when:
  - Worst-case simulation shows no saturation after reducing replicas
  - All remaining replicas stay below saturation thresholds

# Configuration

The engine is configured via the capacity-scaling-config ConfigMap:

	apiVersion: v1
	kind: ConfigMap
	metadata:
	  name: capacity-scaling-config
	data:
	  default: |
	    kvCacheThreshold: 0.80
	    queueLengthThreshold: 5
	    kvSpareTrigger: 0.10
	    queueSpareTrigger: 3

Per-model overrides are supported by adding entries with the model ID as key.

# Usage

The engine is automatically instantiated by the controller when EXPERIMENTAL_PROACTIVE_MODEL
environment variable is set to "false" (default) or omitted:

	env:
	  - name: EXPERIMENTAL_PROACTIVE_MODEL
	    value: "false"  # Use saturation engine only

# Integration

The saturation engine integrates with:

  - MetricsCollector: Gathers replica saturation metrics
  - SaturationAnalyzer: Analyzes capacity and makes scaling decisions
  - Actuator: Emits recommendations via Prometheus metrics
  - HPA/KEDA: Scales deployments based on inferno_desired_replicas metric

# Execution Strategy

The engine uses a polling executor (default: 10s interval) to continuously
monitor and update scaling recommendations. The reconciliation loop:

1. Collects replica metrics from Prometheus
2. Analyzes saturation state per variant
3. Calculates target replica counts
4. Emits metrics to Prometheus
5. Updates VariantAutoscaling status

# Thread Safety

The engine is safe for concurrent use. Internal state is protected by
appropriate synchronization mechanisms.

# Performance

Typical reconciliation time: <5 seconds
Target decision latency: <30 seconds from saturation detection to scale-up

For detailed algorithm information, see docs/saturation-analyzer.md.
*/
package saturation
