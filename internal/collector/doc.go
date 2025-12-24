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
Package collector provides metrics collection functionality for the Workload Variant Autoscaler.

# Overview

The collector package implements a pluggable metrics collection system that gathers
performance and capacity metrics from inference servers. It supports multiple backends
including Prometheus and can be extended to support additional data sources.

# Architecture

The collector provides a factory pattern for creating collector instances:

	collector := factory.NewMetricsCollector(config, client, prometheusAPI)

# Backends

Current implementations:

  - Prometheus: Collects vLLM server metrics via Prometheus queries
  - Cache: Provides caching layer for frequently accessed metrics

# Key Features

  - Pluggable collector backends via factory pattern
  - Background metric fetching with configurable intervals
  - In-memory caching with TTL support
  - Metrics enrichment with pod and deployment metadata
  - Thread-safe concurrent access

# Metrics Collected

The collector gathers saturation-related metrics from inference servers:

  - KV cache utilization percentage
  - Request queue depth
  - Pod and deployment labels for correlation

Metrics are queried using max_over_time aggregations to capture peak values
over configurable time windows (default: 1 minute).

# Usage

Create a collector instance using the factory:

	import (
		"github.com/llm-d-incubation/workload-variant-autoscaler/internal/collector"
		"github.com/llm-d-incubation/workload-variant-autoscaler/internal/collector/config"
	)

	cfg := &config.CollectorConfig{
		Type: "prometheus",
		PrometheusConfig: config.PrometheusConfig{
			Address: "http://prometheus:9090",
		},
	}

	collector, err := factory.NewMetricsCollector(cfg, k8sClient, promAPI)
	if err != nil {
		// handle error
	}

	// Collect metrics for a model
	metrics, err := collector.CollectReplicaMetrics(ctx, modelID, namespace)

# Legacy Functions

Some functions in this package (ValidateMetricsAvailability, AddMetricsToOptStatus)
are deprecated and kept for backward compatibility. New code should use the
MetricsCollector interface from internal/interfaces instead.

# Thread Safety

All collector implementations are safe for concurrent use from multiple goroutines.
The cache implementation uses RWMutex for thread-safe read/write operations.

# Configuration

Collectors are configured via the CollectorConfig structure:

	type CollectorConfig struct {
		Type             string              // "prometheus" or custom
		PrometheusConfig PrometheusConfig    // Prometheus-specific settings
		CacheConfig      CacheConfig         // Cache settings
	}

For detailed configuration options, see internal/collector/config package.
*/
package collector
