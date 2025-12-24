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
Package interfaces defines the core interface contracts for the Workload Variant Autoscaler.

# Overview

This package provides the interface definitions and data structures that enable
pluggable implementations of key WVA components. It serves as the contract layer
between different subsystems of the autoscaler.

# Core Interfaces

MetricsCollector:

Defines the contract for collecting metrics from inference servers:

	type MetricsCollector interface {
		CollectReplicaMetrics(ctx context.Context, modelID, namespace string) ([]ReplicaMetrics, error)
		ValidateMetricsAvailability(ctx context.Context, deployment *appsv1.Deployment) error
		Close()
	}

SaturationAnalyzer:

Defines the contract for saturation-based capacity analysis:

	type SaturationAnalyzer interface {
		AnalyzeModelSaturation(
			ctx context.Context,
			modelID string,
			namespace string,
			replicaMetrics []ReplicaMetrics,
			config SaturationScalingConfig,
		) (*ModelSaturationAnalysis, error)
	}

VariantAutoscalingsEngine:

Defines the contract for optimization engines:

	type VariantAutoscalingsEngine interface {
		Optimize(
			ctx context.Context,
			va VariantAutoscalingList,
			analysis map[string]*ModelAnalyzeResponse,
		) (map[string]OptimizedAlloc, error)
	}

ModelAnalyzer:

Defines the contract for model-based performance analysis using queueing theory:

	type ModelAnalyzer interface {
		AnalyzeModel(ctx context.Context, spec AnalysisSpec) (*ModelAnalysis, error)
	}

# Data Structures

Key data structures defined in this package:

ReplicaMetrics:

Metrics collected from a single inference server replica:

	type ReplicaMetrics struct {
		PodName         string
		VariantName     string
		ModelID         string
		Namespace       string
		AcceleratorType string
		KvCacheUsage    float64  // 0.0-1.0
		QueueLength     int
		VariantCost     float64  // Cost per replica
	}

ModelSaturationAnalysis:

Results of saturation analysis for a model:

	type ModelSaturationAnalysis struct {
		ModelID              string
		Namespace            string
		TotalReplicas        int
		SaturatedReplicas    int
		NonSaturatedReplicas int
		ShouldScaleUp        bool
		ShouldScaleDown      bool
		ScaleDownSafe        bool
		VariantAnalyses      []VariantSaturationAnalysis
	}

SaturationScalingConfig:

Configuration for saturation-based scaling:

	type SaturationScalingConfig struct {
		KvCacheThreshold     float64
		QueueLengthThreshold int
		KvSpareTrigger       float64
		QueueSpareTrigger    int
	}

# Design Principles

Interface Segregation:

Each interface has a single, well-defined responsibility.

Dependency Inversion:

High-level modules (engines, controller) depend on these interfaces,
not on concrete implementations.

Implementation Flexibility:

Multiple implementations can satisfy each interface (e.g., Prometheus
collector, mock collector for testing).

# Package Organization

This package is structured to:

  - Define interface contracts (interfaces.go)
  - Define data transfer objects (types.go)
  - Define domain-specific interfaces (metrics_collector.go, saturation_analyzer.go, etc.)

# Usage

Implementations should satisfy these interfaces:

	import "github.com/llm-d-incubation/workload-variant-autoscaler/internal/interfaces"

	type MyCollector struct {
		// implementation
	}

	// Verify interface compliance at compile time
	var _ interfaces.MetricsCollector = (*MyCollector)(nil)

	func (c *MyCollector) CollectReplicaMetrics(
		ctx context.Context,
		modelID, namespace string,
	) ([]interfaces.ReplicaMetrics, error) {
		// implementation
	}

# Testing

These interfaces enable easy testing through mock implementations:

	type MockCollector struct {
		metrics []interfaces.ReplicaMetrics
		err     error
	}

	func (m *MockCollector) CollectReplicaMetrics(
		ctx context.Context,
		modelID, namespace string,
	) ([]interfaces.ReplicaMetrics, error) {
		return m.metrics, m.err
	}

# Thread Safety

Interface contracts do not mandate thread safety. Implementations should
document their thread-safety guarantees. Most WVA interfaces are designed
for single-threaded usage per instance or with external synchronization.

# Versioning

These interfaces are considered internal and may change between releases.
For stable APIs, use the public API types defined in api/v1alpha1.
*/
package interfaces
