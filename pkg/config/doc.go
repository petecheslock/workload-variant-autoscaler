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
Package config provides configuration types and utilities for WVA.

# Overview

This package defines configuration structures used throughout the Workload
Variant Autoscaler, including model performance parameters, workload
specifications, and service class targets.

# Key Types

ModelAcceleratorPerfData:

Performance parameters for a model on a specific accelerator:

	type ModelAcceleratorPerfData struct {
		Alpha        float64  // Prefill time base (ms)
		Beta         float64  // Prefill time per input token (ms/token)
		Gamma        float64  // Decode time base (ms)
		Delta        float64  // Decode time per output token (ms/token)
		MaxBatchSize int      // Maximum batch size
		MemoryMiB    int      // GPU memory required (MiB)
	}

These parameters are used by the queueing theory models to predict
inference latency and throughput.

ServerLoadSpec:

Workload characteristics for an inference server:

	type ServerLoadSpec struct {
		ArrivalRate  float64  // Requests per second
		AvgInTokens  int      // Average input tokens per request
		AvgOutTokens int      // Average output tokens per request
	}

TargetSpec:

SLO targets for a service class:

	type TargetSpec struct {
		TTFT float64  // Max Time to First Token (ms)
		ITL  float64  // Max Inter-Token Latency (ms)
		TPS  float64  // Min Tokens Per Second
	}

# Configuration Sources

The package supports multiple configuration sources:

ConfigMaps:

Cluster-wide configuration via Kubernetes ConfigMaps:
  - accelerator-unitcost: GPU pricing and capabilities
  - serviceclass: SLO targets and priorities
  - capacity-scaling-config: Saturation thresholds

VariantAutoscaling CRs:

Per-workload configuration in custom resources:
  - modelProfile: Performance parameters
  - serviceClass: Service tier selection
  - variantCost: Cost overrides

# Parameter Estimation

Model performance parameters (alpha, beta, gamma, delta) are obtained
through offline benchmarking. See docs/tutorials/parameter-estimation.md
for detailed guidance on benchmarking procedures.

# Defaults

The package provides sensible defaults:

	DefaultConfig = Config{
		MaxBatchSize:     256,
		DefaultTTFT:      1000.0,  // 1 second
		DefaultITL:       100.0,   // 100ms
		DefaultTPS:       100.0,   // 100 tokens/sec
		ReconcileInterval: 10 * time.Second,
	}

Defaults are used when specific configuration is unavailable.

# Validation

Configuration values are validated on load:

	if config.Alpha < 0 || config.Beta < 0 {
		return nil, fmt.Errorf("performance parameters must be non-negative")
	}

Invalid configurations are rejected with descriptive errors.

# Usage

Load configuration from various sources:

	import "github.com/llm-d-incubation/workload-variant-autoscaler/pkg/config"

	// From ConfigMap
	cfg, err := config.LoadFromConfigMap(cm)

	// From VariantAutoscaling CR
	perfData := config.ExtractPerfData(va.Spec.ModelProfile)

	// Use in optimization
	analyzer.SetConfig(cfg)

# Integration

This package integrates with:

  - pkg/core: Uses config for allocation calculations
  - pkg/analyzer: Uses perf data for queueing models
  - internal/controller: Loads and validates config
  - internal/optimizer: Uses config for optimization

# Thread Safety

Configuration types are immutable after creation and safe for concurrent
read access from multiple goroutines. Loading and parsing operations are
not thread-safe and should be performed during initialization.

# Performance

Configuration loading:
  - ConfigMap parsing: <1ms
  - Validation: <1ms
  - Cache lookup: <1µs

# Best Practices

For configuration management:

  1. Benchmark models offline to obtain accurate parameters
  2. Use ConfigMaps for cluster-wide defaults
  3. Use CR specs for per-workload overrides
  4. Validate configuration during admission control
  5. Provide sensible defaults for optional parameters
  6. Document configuration schema and examples

# Schema Evolution

Configuration schema changes should maintain backward compatibility:

  - Add new fields as optional
  - Preserve existing field semantics
  - Provide migration paths for breaking changes
  - Document deprecation timelines

# Testing

Configuration can be tested in isolation:

	import "testing"

	func TestConfigValidation(t *testing.T) {
		cfg := &Config{
			Alpha: -1.0,  // Invalid
		}
		err := cfg.Validate()
		assert.Error(t, err)
	}

# Examples

See config/samples/ for example configurations demonstrating various
deployment scenarios and parameter combinations.
*/
package config
