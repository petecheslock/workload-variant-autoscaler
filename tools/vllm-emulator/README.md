# vLLM Emulator & Load Generator

This directory contains tools for testing and benchmarking the Workload-Variant-Autoscaler (WVA) without requiring physical GPUs or real LLM deployments.

## Overview

The vLLM emulator provides:
- **Simulated vLLM metrics** compatible with WVA's Prometheus collectors
- **Load generation** for testing autoscaling behavior
- **Configurable behavior** to simulate different workload patterns
- **Local development support** for WVA testing on Kind clusters

## Quick Start

The easiest way to use the emulator is via the Kind deployment:

```bash
# Deploy WVA with emulated environment
make deploy-llm-d-wva-emulated-on-kind
```

See [Kind Emulator README](../../deploy/kind-emulator/README.md) for detailed setup and [vLLM Emulator Example](../../deploy/examples/vllm-emulator/README.md) for deployment instructions.

## Components

### vLLM Metrics Emulator

A lightweight service that emulates vLLM's Prometheus metrics endpoint, emitting metrics like `vllm_cache_usage`, `vllm_queue_depth`, `vllm_requests_total`, and GPU memory metrics.

### Load Generator

A tool for generating synthetic inference requests to test WVA behavior with multiple traffic patterns (constant, ramp, spike, periodic).

### Benchmark Suite

Automated benchmarking for WVA performance testing including scale-up responsiveness, scale-down stability, and cost efficiency tests.

## Documentation

For detailed information, see:
- [Kind Emulator Deployment](../../deploy/kind-emulator/README.md)
- [vLLM Integration Example](../../deploy/examples/vllm-emulator/README.md)
- [Testing Guide](../../docs/developer-guide/testing.md)

---

**Note**: This is a placeholder. Full emulator implementation is part of the deployment examples.
