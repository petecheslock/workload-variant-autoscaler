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
Package solver provides allocation optimization algorithms for inference server autoscaling.

# Overview

The solver package implements algorithms to find optimal resource allocations
across multiple inference servers, variants, and accelerator types. It solves
the allocation assignment problem to minimize cost while meeting SLO requirements.

# Key Components

Solver:
  - Main solver implementing allocation algorithms
  - Maintains current and desired allocations
  - Calculates allocation differences (diffs)
  - Supports multiple solution strategies

Greedy Solver:
  - Fast greedy allocation algorithm
  - Iteratively assigns resources to highest-priority servers
  - Good for online optimization with time constraints

MIP Solver (experimental):
  - Mixed Integer Programming formulation
  - Finds globally optimal solutions
  - Slower but more accurate (future work)

# Allocation Problem

The allocation problem:
  - Given: N servers, M accelerator types, capacity constraints
  - Objective: Minimize total cost
  - Constraints: Meet SLO for each server, respect capacity limits

Formulation:
  - Decision variables: Number of replicas per server per accelerator type
  - Objective function: Sum of (replicas × accelerator_cost)
  - Constraints:
    - Throughput ≥ required rate for each server
    - Latency ≤ SLO for each server
    - Total allocation ≤ capacity for each accelerator type

# Solution Strategies

Unlimited Mode (current):
  - Each variant independently optimized
  - No global capacity constraints
  - Simpler and faster
  - May over-allocate in aggregate

Limited Mode (future):
  - Global optimization with capacity constraints
  - Requires coordination across variants
  - More complex but resource-efficient
  - Handles degraded mode when capacity insufficient

# Greedy Algorithm

The greedy solver algorithm:

	1. Sort servers by priority (higher priority first)
	2. For each server:
	   a. Try each accelerator type (lowest cost first)
	   b. Calculate minimum replicas needed for SLO
	   c. Check capacity availability
	   d. Allocate if capacity sufficient
	   e. Fall back to next accelerator type if insufficient
	3. Return allocation solution

Time complexity: O(N × M × log M) where N=servers, M=accelerator types

# Usage Example

Solving an allocation problem:

	// Create optimizer spec with mode
	spec := &config.OptimizerSpec{
		Unlimited: true,  // Unlimited capacity mode
	}

	// Create solver
	solver := solver.NewSolver(spec)

	// Populate core.System with servers, accelerators, capacity
	// ... (done by optimizer)

	// Solve
	err := solver.Solve()
	if err != nil {
		log.Fatal(err)
	}

	// Extract results
	solution := core.GetAllocationSolution()
	for serverName, alloc := range solution.Allocations {
		log.Info("Allocation", "server", serverName,
			"accelerator", alloc.Accelerator,
			"replicas", alloc.Replicas)
	}

# Allocation Solution

The solution includes:
  - Allocations: Per-server allocation (accelerator type, replicas)
  - Total cost: Sum of allocation costs
  - Feasibility: Whether all constraints satisfied
  - Metadata: Optimization timestamp, algorithm used

Solution structure:

	type AllocationSolution struct {
		Allocations  map[string]*Allocation
		TotalCost    float64
		Feasible     bool
		Algorithm    string
		OptimizedAt  time.Time
	}

# Allocation Diffs

The solver tracks allocation changes:
  - Current allocation: Existing replica counts
  - Desired allocation: Calculated by solver
  - Diff: Change needed (scale up/down/no change)

Diff types:
  - ScaleUp: Increase replicas
  - ScaleDown: Decrease replicas
  - NoChange: Maintain current allocation
  - NewAllocation: First-time allocation

# Cost Calculation

Cost is calculated as:

	cost = accelerator_cost × replicas × time_unit

Where:
  - accelerator_cost: Cost per accelerator per time unit (e.g., $/hour)
  - replicas: Number of replicas allocated
  - time_unit: Typically 1 hour

Multi-accelerator allocations:

	cost = accelerator_cost × accelerator_count × replicas × time_unit

# Performance Sizing

The solver uses performance models to size allocations:
  - pkg/analyzer: Queueing models for latency/throughput
  - Inputs: Request rate, token counts, SLO targets
  - Outputs: Required batch size, replicas, accelerator type

Sizing flow:
  1. Estimate single-replica capacity using performance model
  2. Calculate replicas needed: ceil(request_rate / replica_capacity)
  3. Validate SLO compliance with calculated allocation
  4. Adjust if needed

# Service Class Priorities

Servers are prioritized by service class:
  - Priority 1: Most critical (allocated first)
  - Priority 2, 3, ...: Less critical
  - When capacity insufficient, lower priority servers may not get allocation

Criticality:
  - Used for admission control decisions
  - Higher criticality = less likely to be dropped
  - Range: 0.0 (can drop) to 1.0 (never drop)

# Capacity Management

The solver respects capacity constraints:
  - Available capacity per accelerator type
  - Already-allocated capacity
  - Remaining capacity for new allocations

Capacity tracking:

	available[accel] = total_capacity[accel] - allocated[accel]

When capacity insufficient:
  - Unlimited mode: Proceed anyway (assume elastic cluster)
  - Limited mode: Leave servers unallocated, mark infeasible

# Error Handling

The solver handles:
  - No feasible solution: Return error, mark infeasible
  - Capacity exceeded: Either allow (unlimited) or fail (limited)
  - Invalid parameters: Validate before solving
  - Model errors: Propagate from performance analyzer

# Optimization Metadata

The solver tracks metadata:
  - Optimization timestamp
  - Algorithm used (greedy, MIP, etc.)
  - Iterations performed
  - Convergence status
  - Warnings and degraded conditions

# Testing

The solver has comprehensive tests:
  - Unit tests: Algorithm correctness
  - Integration tests: End-to-end optimization
  - Performance tests: Large-scale scenarios
  - Property tests: Cost monotonicity, feasibility

# Future Enhancements

Planned improvements:
  - MIP solver implementation
  - Multi-objective optimization (cost + latency)
  - Predictive scaling based on traffic forecasts
  - GPU disaggregation support (prefill/decode)
  - Incremental optimization (reuse previous solutions)

# Integration Points

Integrates with:
  - pkg/core: Uses core domain models
  - pkg/analyzer: Performance models for sizing
  - internal/optimizer: Called by optimizer
  - internal/engines: Provides allocation to engines

# See Also

For core domain models, see pkg/core
For queueing models, see pkg/analyzer
For optimization coordination, see internal/optimizer
For architecture overview, see docs/architecture.md
*/
package solver
