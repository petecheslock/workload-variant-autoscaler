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
Package manager provides system state management for the model-based optimizer.

# Overview

This package implements the system state manager that coordinates the lifecycle
of optimization components. It maintains the global state of servers, models,
accelerators, and service classes used by the model-based optimization engine.

# Architecture

The manager provides a central coordination point:

	VariantAutoscaling CRs → Manager → System State → Optimizer

Key responsibilities:

  1. Initialize system state from VariantAutoscaling resources
  2. Coordinate between optimizer, analyzer, and solver
  3. Manage lifecycle of optimization components
  4. Handle state transitions and updates
  5. Provide query interfaces for system state

# System State

The manager maintains global state through the System singleton:

	system := core.GetSystem()

System state includes:

  - Servers: Inference server instances with load specs
  - Models: AI models with performance data
  - Accelerators: GPU types with costs and capacities
  - ServiceClasses: Service tiers with SLO targets

# Lifecycle Management

The manager handles component lifecycle:

Initialization:

	manager := manager.NewManager()
	manager.Initialize(ctx, variantList)

This creates servers, registers models, and sets up service classes.

Update:

	manager.Update(ctx, variantList)

This updates system state to reflect changes in VariantAutoscaling resources.

Cleanup:

	manager.Cleanup()

This removes servers and clears state for deleted resources.

# Integration

The manager integrates with:

  - pkg/core: System state and domain types
  - pkg/solver: Provides state for optimization
  - internal/optimizer: Coordinates optimization execution
  - internal/engines/model: Model-based engine coordination

# State Queries

The manager provides query interfaces:

	// Get all servers
	servers := manager.GetServers()

	// Get server by name
	server := manager.GetServer("llama-8b-server")

	// Get models
	models := manager.GetModels()

	// Get available accelerators
	accelerators := manager.GetAccelerators()

# Configuration Loading

The manager loads configuration from multiple sources:

ConfigMaps:

	manager.LoadAcceleratorConfig(configMap)
	manager.LoadServiceClassConfig(configMap)

VariantAutoscaling CRs:

	manager.SyncWithVariants(variantList)

# Error Handling

The manager handles errors gracefully:

Missing Configuration:

Falls back to defaults and logs warnings.

Invalid Data:

Skips invalid entries and continues processing valid ones.

State Conflicts:

Resolves conflicts using last-write-wins with timestamp tracking.

# Thread Safety

The manager is NOT thread-safe. It should be used from a single goroutine
(typically the optimization engine's execution thread). For concurrent access,
external synchronization is required.

# Performance

State operations:
  - Initialize with 20 variants: <10ms
  - Update single variant: <1ms
  - Query operations: <100µs

# Usage

Create and use the manager:

	import "github.com/llm-d-incubation/workload-variant-autoscaler/pkg/manager"

	mgr := manager.NewManager()

	// Initialize with VariantAutoscaling resources
	err := mgr.Initialize(ctx, variantList)
	if err != nil {
		// handle error
	}

	// Query state
	servers := mgr.GetServers()
	for _, server := range servers {
		fmt.Printf("Server: %s, Model: %s\n",
			server.Name(), server.ModelName())
	}

	// Update state
	err = mgr.Update(ctx, updatedVariantList)

	// Cleanup
	mgr.Cleanup()

# Testing

The manager supports test modes:

	mgr := manager.NewManager()
	mgr.SetTestMode(true)
	mgr.SetMockSystem(mockSystem)

# Best Practices

For manager usage:

  1. Initialize once at engine startup
  2. Update on configuration changes
  3. Clean up on engine shutdown
  4. Use queries instead of direct system access
  5. Handle errors during initialization

# Relationship to System

The manager wraps the System singleton and provides higher-level
coordination logic. Direct System access should be avoided; use
the manager's interfaces instead.

# Status

The manager is part of the experimental model-based engine. It is
not used in the default saturation-based engine mode.
*/
package manager
