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
Package solver implements optimization algorithms for resource allocation in WVA.

# Overview

This package provides optimization solvers that determine the best allocation
of GPU resources to inference servers while minimizing cost and meeting SLO
requirements. It is used by the model-based optimization engine in hybrid mode.

# Algorithms

Greedy Solver:

A greedy algorithm that allocates accelerators to servers based on priority
and cost efficiency:

	solver := solver.NewSolver()
	solver.SolveGreedy()

The algorithm:
  1. Sorts servers by service class priority
  2. For each server, evaluates all feasible accelerator allocations
  3. Selects allocation with best cost/performance trade-off
  4. Respects cluster capacity constraints
  5. Ensures high-priority servers get allocations first

Optimization Solver:

An optimization-based solver using integer programming or constraint solving:

	solver.SolveOptimization()

This solver:
  - Finds globally optimal solutions
  - Handles complex constraint relationships
  - Minimizes total cost while meeting all SLOs
  - More computationally expensive than greedy

# Operating Modes

Limited Mode (Capacity-Constrained):

The solver operates with finite cluster capacity:

	capacities := map[string]int{
		"A100": 8,
		"H100": 4,
	}
	system.SetCapacities(capacities)

In this mode, the solver must fit all allocations within available resources.

Unlimited Mode:

Each variant receives optimal allocation independently:

	system.SetCapacities(nil)  // Unlimited capacity

Allocations are determined purely by workload requirements and SLO targets,
without cluster capacity constraints.

# Allocation Strategy

The solver considers multiple factors when allocating:

Priority:

Service classes have priority levels. Higher priority workloads get
resources first:

	type ServiceClass struct {
		priority int  // 1=highest
	}

Cost:

Accelerator costs influence allocation decisions:

	type Accelerator struct {
		cost float32  // $/hour
	}

Feasibility:

Allocations must meet SLO requirements:

	if allocation.ttft > target.TTFT {
		// Allocation is infeasible
	}

# Data Structures

ServerEntry:

Internal structure for solver state:

	type serverEntry struct {
		serverName  string
		priority    int
		allocations []*core.Allocation
		delta       float32
	}

Delta values represent the cost penalty of moving to the next allocation
option, enabling greedy selection.

# Algorithm Details

Greedy Solver Steps:

	1. Create entries for all servers with sorted allocation options
	2. While servers remain unallocated:
	   a. Select server with highest priority and lowest delta
	   b. Try to allocate current best option
	   c. If capacity available, allocate and continue
	   d. Otherwise, try next option or mark infeasible
	3. Return allocation results

# Integration

This package integrates with:

  - pkg/core: Uses allocations and system state
  - pkg/manager: Coordinates optimization execution
  - internal/engines/model: Model-based optimization engine

# Usage

	import (
		"github.com/llm-d-incubation/workload-variant-autoscaler/pkg/solver"
		"github.com/llm-d-incubation/workload-variant-autoscaler/pkg/core"
	)

	// Set up system with servers, models, accelerators
	system := core.GetSystem()
	// ... configure system ...

	// Create solver and optimize
	solver := solver.NewSolver()
	solver.SolveGreedy()

	// Check results
	for _, server := range system.GetServers() {
		if allocation := server.GetAllocation(); allocation != nil {
			fmt.Printf("Server %s: %d x %s replicas\n",
				server.Name(),
				allocation.NumReplicas(),
				allocation.Accelerator())
		}
	}

# Performance

Greedy solver performance:
  - 10 servers, 3 accelerator types: <10ms
  - 100 servers, 5 accelerator types: <100ms

Optimization solver performance:
  - 10 servers: <1s
  - 100 servers: 10-60s depending on complexity

# Thread Safety

The Solver type is not thread-safe. External synchronization is required
if accessed from multiple goroutines.

# Limitations

The solver operates in limited mode only when integrated with the llm-d stack.
Standalone deployments use unlimited mode by default.

For production use, consider the saturation-based engine (internal/engines/saturation)
which provides faster response times (<30s) without requiring model training.
*/
package solver
