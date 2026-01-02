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
Package saturation provides saturation-based capacity analysis for inference server autoscaling.

# Overview

The saturation package implements the core logic for analyzing vLLM server metrics
to determine scaling decisions based on capacity utilization. It prevents capacity
exhaustion by monitoring KV cache usage and request queue lengths.

# Key Components

The Analyzer performs saturation analysis:
  - Classifies replicas as saturated or non-saturated based on thresholds
  - Calculates spare capacity across non-saturated replicas
  - Determines scale-up triggers when capacity exhaustion is imminent
  - Validates scale-down safety through worst-case simulation
  - Makes per-variant scaling decisions with cost-awareness

# Usage Example

	analyzer := saturation.NewAnalyzer()

	analysis, err := analyzer.AnalyzeModelSaturation(
		ctx,
		"meta/llama-3.1-8b",
		"llm-inference",
		replicaMetrics,
		config,
	)

	if err != nil {
		log.Fatal(err)
	}

	if analysis.ShouldScaleUp {
		log.Info("Scaling up due to capacity exhaustion risk")
	}

# Thresholds

Default saturation thresholds (configurable via SaturationScalingConfig):
  - KV Cache: 80% - Scale if KV cache usage exceeds this
  - Queue Length: 5 requests - Scale if queue length exceeds this
  - Trigger: 20% - Scale up if spare capacity falls below this
  - Target: 40% - Target spare capacity after scale-up

# Algorithm

The saturation analysis follows these steps:

1. **Classify Replicas**: Determine which replicas are saturated
   - Saturated: kv_cache >= threshold OR queue_length >= threshold
   - Non-saturated: Both metrics below thresholds

2. **Calculate Spare Capacity**: For non-saturated replicas
   - Spare = min(1 - kv_cache_usage, 1 - queue_length/threshold)
   - Average spare capacity across all non-saturated replicas

3. **Scale-Up Decision**:
   - Trigger if average spare capacity < trigger threshold (default 20%)
   - Calculate replicas needed to reach target spare capacity (default 40%)

4. **Scale-Down Safety**:
   - Simulate worst-case: Remove one replica
   - Check if remaining capacity meets trigger threshold
   - Only allow scale-down if simulation passes

5. **Per-Variant Allocation**:
   - Distribute additional replicas across variants
   - Prefer lower-cost accelerators when available
   - Preserve existing allocations when possible

# Modes

Capacity-Only Mode (current):
  - Pure saturation-based scaling
  - No offline profiling required
  - Fast and reactive

Hybrid Mode (future):
  - Combine with model-based optimizer decisions
  - Saturation provides safety guardrails
  - Model-based optimizer handles SLO optimization

# Data Structures

Key interfaces (defined in internal/interfaces):
  - ReplicaMetrics: Per-replica saturation metrics
  - ModelSaturationAnalysis: Analysis results with scaling decisions
  - VariantSaturationAnalysis: Per-variant breakdown
  - SaturationScalingConfig: Configuration thresholds

# Integration

The saturation analyzer is called from the saturation engine:
  - internal/engines/saturation: Saturation-based scaling engine
  - internal/controller: Controller reconciliation loop

Metrics are collected by:
  - internal/collector: Metrics collection from Prometheus

Results are used by:
  - internal/actuator: Emits desired_replicas metric

# See Also

For detailed algorithm explanation, see docs/saturation-analyzer.md
For architecture overview, see docs/architecture.md
*/
package saturation
