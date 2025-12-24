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
Package optimizer provides optimization coordination for the model-based engine.

# Overview

This package implements the optimizer component that coordinates between the
model analyzer and the solver to determine optimal resource allocations. It
serves as the orchestration layer for the model-based optimization pipeline.

# Architecture

The optimizer coordinates three main components:

  1. Model Analyzer: Evaluates performance for candidate allocations
  2. Solver: Finds optimal allocations across all servers
  3. System State: Manages servers, models, and accelerators

The optimization flow:

	Workload Metrics → Optimizer → Model Analyzer → Solver → Allocations

# Key Responsibilities

Workload Translation:

Converts VariantAutoscaling resources into optimization problem:

	- Extract arrival rates and token lengths from metrics
	- Build server instances with load specifications
	- Configure service classes and SLO targets

Model Setup:

Prepares the optimization environment:

	- Register models with performance data
	- Register accelerators with costs and capacities
	- Create servers for each variant

Optimization Execution:

Runs the optimization process:

	- Invoke solver to find allocations
	- Validate results against SLO targets
	- Generate scaling recommendations

Result Translation:

Converts allocations to WVA recommendations:

	- Map allocations to desired replica counts
	- Calculate cost and performance metrics
	- Update VariantAutoscaling status

# Integration

The optimizer integrates with:

  - pkg/core: Domain types and allocation logic
  - pkg/solver: Optimization algorithms
  - pkg/analyzer: Queueing theory models
  - internal/engines/model: Model-based engine
  - internal/collector: Metrics collection

# Usage

The optimizer is used internally by the model-based engine:

	import "github.com/llm-d-incubation/workload-variant-autoscaler/internal/optimizer"

	opt := optimizer.NewOptimizer()
	results, err := opt.Optimize(ctx, variantList, metricsMap)

# Configuration

The optimizer uses configuration from multiple sources:

ConfigMaps:
  - accelerator-unitcost: GPU pricing information
  - serviceclass: SLO targets and priorities

VariantAutoscaling CRs:
  - modelProfile: Performance parameters
  - serviceClass: Service tier assignment
  - variantCost: Cost overrides

# Operating Modes

Limited Mode:

Optimizes within cluster capacity constraints:

	capacities := map[string]int{"A100": 8, "H100": 4}
	optimizer.SetCapacities(capacities)

The solver ensures total allocations fit within available resources.

Unlimited Mode (Default):

Each variant receives optimal allocation independently:

	optimizer.SetCapacities(nil)

No cluster capacity constraints are enforced.

# Performance

Optimization time varies with problem size:

  - 5 variants, 3 accelerators: <1 second
  - 20 variants, 5 accelerators: 2-4 seconds
  - 50 variants, 10 accelerators: 5-10 seconds

The optimizer implements caching to avoid redundant calculations:

  - Model performance data cached per accelerator
  - Feasibility checks cached per allocation
  - System state cached across optimization cycles

# Error Handling

The optimizer handles various error conditions:

Infeasible Problems:

When no allocation can meet SLO targets:

	- Returns error with detailed diagnosis
	- Identifies which constraints are violated
	- Suggests configuration adjustments

Missing Data:

When required configuration is unavailable:

	- Falls back to default values where possible
	- Returns error for critical missing data
	- Logs warnings for optional missing data

# Validation

The optimizer validates:

  - Model profiles are complete and consistent
  - Accelerator configurations are valid
  - Service class targets are achievable
  - Workload metrics are within reasonable bounds

Validation errors are returned early to avoid wasted optimization time.

# Thread Safety

The Optimizer type is safe for single-threaded usage per instance. For
concurrent optimization of multiple variant sets, create separate optimizer
instances.

# Testing

The optimizer supports test modes:

Mock Mode:

Uses mock implementations for testing:

	opt := optimizer.NewOptimizer()
	opt.SetTestMode(true)
	opt.SetMockCollector(mockCollector)

Dry Run:

Validates configuration without emitting recommendations:

	results, err := opt.OptimizeDryRun(ctx, variantList)

# Metrics

The optimizer emits internal metrics:

  - Optimization duration (histogram)
  - Allocation feasibility rate (counter)
  - Cost reduction achieved (gauge)

These are logged but not exposed as Prometheus metrics.

# Status

The optimizer is part of the experimental model-based engine. For production
deployments, use the saturation-based engine instead, which does not require
the optimizer component.
*/
package optimizer
