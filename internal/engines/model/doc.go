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
Package model implements the model-based optimization engine for WVA.

# Overview

The model engine provides proactive, model-based scaling decisions using queueing
theory to predict inference server performance. This engine is experimental and
available in hybrid mode alongside the saturation analyzer.

# Key Features

  - Proactive scaling based on predicted performance
  - Uses queueing theory models (M/M/1/k, state-dependent models)
  - Considers arrival rates, token lengths, and SLO targets
  - Provides global optimization across multiple models and service classes
  - Integrates with pkg/core and pkg/solver for allocation optimization

# Architecture

The model engine operates on a polling schedule (configurable interval):

	1. Collect current workload metrics (arrival rates, token lengths)
	2. Build optimization problem with servers, models, and accelerators
	3. Run solver to find optimal allocations
	4. Emit scaling recommendations

# Usage

The engine is activated when EXPERIMENTAL_PROACTIVE_MODEL is set to "true":

	env:
	  - name: EXPERIMENTAL_PROACTIVE_MODEL
	    value: "true"

In hybrid mode, the model engine works alongside the saturation analyzer:
  - Saturation analyzer provides reactive safety guardrails
  - Model engine provides proactive optimization
  - Arbitration logic selects the appropriate recommendation

# Performance Modeling

The engine uses performance parameters to predict latency and throughput:

	type ModelAcceleratorPerfData struct {
		Alpha float64  // Prefill time coefficient
		Beta  float64  // Prefill time per input token
		Gamma float64  // Decode time base
		Delta float64  // Decode time per output token
	}

These parameters are obtained through offline benchmarking. See
docs/tutorials/parameter-estimation.md for guidance.

# Integration

The model engine integrates with:

  - pkg/analyzer: Queueing theory models for performance prediction
  - pkg/core: Allocation logic and domain types
  - pkg/solver: Optimization algorithms
  - internal/collector: Workload metrics collection
  - internal/interfaces: Engine interface contracts

# Execution Strategy

The model engine uses the polling executor with configurable interval:

	defaultPollingInterval = 15 * time.Second

Each execution cycle:
  1. Takes 1-5 seconds for optimization (depends on problem size)
  2. Emits updated recommendations via Prometheus metrics
  3. Waits for next polling interval

Total decision latency: 15-60 seconds from workload change to recommendation.

# Limitations

The model engine requires:

  - Offline benchmarking for performance parameters
  - Stable traffic patterns for accurate predictions
  - Model profile configuration in VariantAutoscaling CRs

For architectures like MoE or HSSM, model predictions may be less accurate.
See docs/design/architecture-limitations.md for details.

# Configuration

Model profiles are specified in VariantAutoscaling resources:

	apiVersion: llmd.ai/v1alpha1
	kind: VariantAutoscaling
	spec:
	  modelID: "meta/llama-3.1-8b"
	  modelProfile:
	    acceleratorProfiles:
	      - acceleratorType: "A100"
	        alpha: 10.5
	        beta: 0.024
	        gamma: 2.1
	        delta: 0.008
	        maxBatchSize: 256

# Hybrid Mode Arbitration

When both engines are active, arbitration follows these rules:

  1. If saturation detected, use saturation recommendation (safety first)
  2. If model predicts need for more capacity, use model recommendation
  3. For scale-down, validate with saturation analyzer (safety check)
  4. Prefer capacity safety over model predictions when in conflict

# Thread Safety

The model engine is safe for single-threaded usage per instance. The polling
executor manages execution scheduling and prevents concurrent optimization runs.

# Performance

Typical optimization time:
  - 5 models, 3 accelerators: 1-2 seconds
  - 20 models, 5 accelerators: 3-5 seconds

# Comparison with Saturation Engine

Model Engine:
  - Proactive (predicts future needs)
  - Requires parameter estimation
  - Slower response (15-60s)
  - Experimental status

Saturation Engine (Recommended):
  - Reactive (responds to current state)
  - No parameter estimation needed
  - Fast response (<30s)
  - Production-ready

For most deployments, use CAPACITY-ONLY mode with the saturation engine only.
*/
package model
