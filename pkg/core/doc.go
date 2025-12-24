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
Package core implements the core domain types and allocation logic for WVA's model-based optimizer.

# Overview

This package provides the foundational types and algorithms for the model-based
optimization engine. It implements queueing theory models to analyze inference
server performance and determine optimal resource allocations.

# Core Types

Accelerator:

Represents a GPU/accelerator type with its characteristics:

	type Accelerator struct {
		name         string
		cost         float32
		memory       int
		computeUnits int
	}

Model:

Represents an AI model with performance characteristics on different accelerators:

	type Model struct {
		name     string
		perfData map[string]*config.ModelAcceleratorPerfData
	}

Server:

Represents an inference server instance serving a model:

	type Server struct {
		name             string
		modelName        string
		serviceClassName string
		load             *config.ServerLoadSpec
		allocation       *Allocation
	}

ServiceClass:

Represents a service tier with SLO requirements:

	type ServiceClass struct {
		name        string
		priority    int
		targets     map[string]*Target  // Per-model targets
	}

# Allocation Logic

The package implements allocation feasibility checking and optimization:

Create Allocation:

	allocation := CreateAllocation(serverName, acceleratorName)

This function:
  - Validates server and accelerator compatibility
  - Uses queueing theory models (M/M/1/k) for performance prediction
  - Checks if SLO targets can be met
  - Calculates optimal batch size and replica count
  - Returns nil if allocation is infeasible

Performance Metrics Calculated:

  - Average response time (latency)
  - Time to First Token (TTFT)
  - Inter-Token Latency (ITL)
  - Throughput and utilization

# Queueing Theory Integration

The package integrates with pkg/analyzer for queueing models:

  - M/M/1/k: Basic queue with finite capacity
  - State-dependent models: Variable service rates based on batch size

Performance is validated against SLO targets:

	target.TTFT  // Max time to first token (ms)
	target.ITL   // Max inter-token latency (ms)
	target.TPS   // Min tokens per second

# System-Wide View

The System type provides global state management:

	system := GetSystem()
	system.AddServer(server)
	system.AddAccelerator(accelerator)
	servers := system.GetServers()

# Cost Optimization

Allocations include cost calculations for optimization:

	allocation.cost = replicaCount * acceleratorCost

The solver uses these costs to minimize total infrastructure cost while
meeting all SLO requirements.

# Zero-Load Handling

Special handling for zero-load scenarios:

	if load.ArrivalRate == 0 || load.AvgOutTokens == 0 {
		return zeroLoadAllocation(...)
	}

Zero-load allocations maintain minimum viable configuration without
running expensive performance calculations.

# Batch Size Calculation

Optimal batch size is determined by:

  - Model architecture characteristics
  - GPU memory constraints
  - Request arrival patterns
  - Average request length

The package ensures batch sizes fit within GPU memory limits and
align with model performance characteristics.

# Usage

Initialize the system with models and accelerators:

	import "github.com/llm-d-incubation/workload-variant-autoscaler/pkg/core"

	// Register accelerators
	acc := core.NewAccelerator("A100", 40.0, 80*1024, 108)
	system.AddAccelerator(acc)

	// Register models
	model := core.NewModel("meta/llama-3.1-8b")
	model.AddPerfData("A100", perfData)
	system.AddModel(model)

	// Create servers
	server := core.NewServer("llama-8b-server", "meta/llama-3.1-8b", "Premium")
	server.SetLoad(arrivalRate, avgInTokens, avgOutTokens)
	system.AddServer(server)

	// Calculate allocation
	allocation := core.CreateAllocation("llama-8b-server", "A100")
	if allocation != nil {
		server.SetAllocation(allocation)
	}

# Integration

This package is used by:

  - pkg/solver: Uses allocations for optimization
  - pkg/manager: Manages system state
  - internal/engines/model: Model-based optimization engine

# Thread Safety

The System singleton uses internal synchronization for thread-safe access.
Individual types (Allocation, Server, Model) are not thread-safe and should
be accessed with appropriate synchronization when shared.

# Performance Characteristics

Allocation creation: ~1-5ms per allocation
Queueing model evaluation: <1ms per model

For experimental/hybrid mode only. Production deployments should use the
saturation-based engine (internal/engines/saturation) instead.
*/
package core
