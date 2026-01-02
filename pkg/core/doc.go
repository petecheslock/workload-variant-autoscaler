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
Package core provides core domain models for the inference server autoscaling system.

# Overview

The core package defines fundamental abstractions representing the system's entities
and their relationships. These models form the foundation for optimization algorithms
and allocation decisions.

# Key Abstractions

System:
  - Global state container for all system entities
  - Manages accelerators, models, servers, and service classes
  - Tracks capacity and allocation across accelerator types
  - Maintains allocation solutions from optimization

Accelerator:
  - Represents a GPU type (e.g., A100, H100, L40S)
  - Defines cost per unit time
  - Specifies multiplicity (number of GPUs per unit)
  - Used for cost-aware allocation decisions

Model:
  - Represents an LLM model (e.g., Llama-3.1-8B)
  - Contains performance characteristics
  - May have different profiles per accelerator type
  - Used for performance modeling and sizing

Server:
  - Represents an inference server instance (replica)
  - Has an allocation of accelerators and capacity
  - Belongs to a service class
  - Tracks current and desired allocation

ServiceClass:
  - Defines QoS requirements (SLOs)
  - Specifies priority for resource allocation
  - Groups servers serving similar workloads
  - Used for prioritization in limited capacity mode

Allocation:
  - Represents resource allocation for a server
  - Specifies accelerator type, count, and replicas
  - Tracks capacity metrics (max batch size, request rate)
  - Supports allocation comparison and updates

# Domain Model Hierarchy

	System
	├── Accelerators
	│   └── Accelerator (type, cost, multiplicity)
	├── Models
	│   └── Model (ID, performance profile)
	├── ServiceClasses
	│   └── ServiceClass (SLO, priority, criticality)
	└── Servers
	    └── Server (name, allocation, service class)
	        └── Allocation (accelerator, replicas, capacity)

# Usage Example

Creating a system with accelerators and servers:

	system := core.NewSystem()

	// Add accelerator type
	a100 := core.NewAccelerator("A100", 1, 1.0)
	system.AddAccelerator(a100)

	// Add model
	model := core.NewModel("meta/llama-3.1-8b")
	system.AddModel(model)

	// Add service class
	sc := core.NewServiceClass("premium", 1, 0.95)
	sc.SetSLO(10, 1000) // TPOT=10ms, TTFT=1000ms
	system.AddServiceClass(sc)

	// Create server with allocation
	server := core.NewServer("vllm-1", "meta/llama-3.1-8b", "premium")
	alloc := core.NewAllocation("A100", 1, 3) // 1 A100, 3 replicas
	server.SetAllocation(alloc)
	system.AddServer(server)

	// Query system state
	capacity := system.GetCapacity("A100")
	servers := system.GetServers()

# Allocation Management

Allocations represent resource assignments:

Creation:
  - NewAllocation(accelerator, count, replicas)
  - Allocate specific GPU type and replica count

Updating:
  - SetAllocation(allocation) - Update server allocation
  - UpdateReplicas(delta) - Increment/decrement replicas
  - ResetAllocation() - Clear allocation

Comparison:
  - IsSame(other) - Compare two allocations
  - Cost() - Calculate allocation cost
  - Capacity() - Get allocation capacity metrics

# Service Classes

Service classes define QoS tiers:

Creation:
  - NewServiceClass(name, priority, criticality)
  - Priority: Higher = more important (1 = highest)
  - Criticality: 0.0 to 1.0, used for admission control

SLO Definition:
  - SetSLO(tpot, ttft) - Time per output token (ms), Time to first token (ms)
  - GetSLO() - Retrieve SLO targets

Usage:
  - Group servers with similar requirements
  - Prioritize allocation in limited capacity mode
  - Validate SLO compliance

# Capacity Tracking

The system tracks capacity at multiple levels:

Accelerator Capacity:
  - Total available count per accelerator type
  - Allocated count per type
  - Remaining capacity

Server Capacity:
  - Max batch size
  - Max request rate per replica
  - Throughput and latency characteristics

Allocation Capacity:
  - Derived from accelerator and replica count
  - Used for sizing and validation

# Operating Modes

Unlimited Mode (current):
  - Each variant gets optimal allocation independently
  - No global capacity constraints
  - Simpler optimization

Limited Mode (future):
  - Global optimization with cluster capacity constraints
  - Requires capacity tracking and reservation
  - Complex allocation algorithms

# Optimization Integration

The core models integrate with:
  - pkg/solver: Optimization algorithms
  - internal/optimizer: Optimization coordination
  - internal/engines: Scaling engines

Optimization flow:
  1. Populate System with current state
  2. Run Solver.Solve() to find allocation
  3. Extract AllocationSolution from System
  4. Apply allocation updates to servers

# Persistence

The core models are ephemeral (in-memory):
  - Created fresh each reconciliation cycle
  - Populated from Kubernetes resources
  - Not persisted to disk or database

This simplifies the design and avoids stale state issues.

# Thread Safety

The core models are NOT thread-safe:
  - Designed for single-threaded controller reconciliation
  - No concurrent access expected
  - External synchronization required for concurrent use

# Testing

The core package has extensive test coverage:
  - Unit tests for each type (allocation_test.go, server_test.go, etc.)
  - Table-driven tests for complex scenarios
  - Property-based tests for allocation logic

# Data Structures

Key data structures:
  - System.accelerators: map[string]*Accelerator
  - System.models: map[string]*Model
  - System.servers: map[string]*Server
  - System.serviceClasses: map[string]*ServiceClass
  - System.capacity: map[string]int
  - System.allocationByType: map[string]*AllocationByType

# Constants

Defined constants:
  - DefaultMaxBatchSize: Default batch size if not specified
  - DefaultMaxReplicas: Default replica limit
  - DefaultPriority: Default service class priority

# Validation

The package provides validation methods:
  - ValidateAccelerator(acc) - Check accelerator definition
  - ValidateAllocation(alloc) - Check allocation constraints
  - ValidateServiceClass(sc) - Check service class definition

# Error Handling

Errors are returned for:
  - Invalid parameters (nil, negative values)
  - Resource not found (accelerator, model, server)
  - Constraint violations (capacity exceeded)

# Integration Points

Integrates with:
  - pkg/solver: Uses core models for optimization
  - pkg/analyzer: Provides model parameters
  - internal/optimizer: Orchestrates core model population
  - internal/utils: Utility functions for model conversion

# See Also

For optimization algorithms, see pkg/solver
For system architecture, see docs/architecture.md
For developer guide, see docs/developer-guide/development.md
*/
package core
