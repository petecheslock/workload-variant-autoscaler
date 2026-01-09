# API Reference

## Packages
- [llmd.ai/v1alpha1](#llmdaiv1alpha1)


## llmd.ai/v1alpha1

Package v1alpha1 contains API Schema definitions for the llmd v1alpha1 API group.

### Resource Types
- [VariantAutoscaling](#variantautoscaling)
- [VariantAutoscalingList](#variantautoscalinglist)



#### ActuationStatus



ActuationStatus provides details about the actuation process and its current status.



_Appears in:_
- [VariantAutoscalingStatus](#variantautoscalingstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `applied` _boolean_ | Applied indicates whether the actuation was successfully applied. |  |  |


#### OptimizedAlloc



OptimizedAlloc describes the target optimized allocation for a model variant.



_Appears in:_
- [VariantAutoscalingStatus](#variantautoscalingstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `lastRunTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#time-v1-meta)_ | LastRunTime is the timestamp of the last optimization run. |  |  |
| `accelerator` _string_ | Accelerator is the type of accelerator for the optimized allocation. |  | MinLength: 2 <br /> |
| `numReplicas` _integer_ | NumReplicas is the number of replicas for the optimized allocation. |  | Minimum: 1 <br /> |


#### VariantAutoscaling



VariantAutoscaling is the Schema for the variantautoscalings API.
It represents the autoscaling configuration and status for a model variant.



_Appears in:_
- [VariantAutoscalingList](#variantautoscalinglist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `llmd.ai/v1alpha1` | | |
| `kind` _string_ | `VariantAutoscaling` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[VariantAutoscalingSpec](#variantautoscalingspec)_ | Spec defines the desired state for autoscaling the model variant. |  |  |
| `status` _[VariantAutoscalingStatus](#variantautoscalingstatus)_ | Status represents the current status of autoscaling for the model variant. |  |  |


#### VariantAutoscalingList



VariantAutoscalingList contains a list of VariantAutoscaling resources.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `llmd.ai/v1alpha1` | | |
| `kind` _string_ | `VariantAutoscalingList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[VariantAutoscaling](#variantautoscaling) array_ | Items is the list of VariantAutoscaling resources. |  |  |


#### VariantAutoscalingSpec



VariantAutoscalingSpec defines the desired state for autoscaling a model variant.



_Appears in:_
- [VariantAutoscaling](#variantautoscaling)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `scaleTargetRef` _[CrossVersionObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#crossversionobjectreference-v1-autoscaling)_ | ScaleTargetRef references the scalable resource to manage.<br />This follows the same pattern as HorizontalPodAutoscaler. |  | Required: \{\} <br /> |
| `modelID` _string_ | ModelID specifies the unique identifier of the model to be autoscaled. |  | MinLength: 1 <br />Required: \{\} <br /> |
| `variantCost` _string_ | VariantCost specifies the cost per replica for this variant (used in saturation analysis). | 10.0 | Optional: \{\} <br />Pattern: `^\d+(\.\d+)?$` <br /> |


#### VariantAutoscalingStatus



VariantAutoscalingStatus represents the current status of autoscaling for a variant,
including the desired optimized allocation and actuation status.



_Appears in:_
- [VariantAutoscaling](#variantautoscaling)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `desiredOptimizedAlloc` _[OptimizedAlloc](#optimizedalloc)_ | DesiredOptimizedAlloc indicates the target optimized allocation based on autoscaling logic. |  |  |
| `actuation` _[ActuationStatus](#actuationstatus)_ | Actuation provides details about the actuation process and its current status. |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta) array_ | Conditions represent the latest available observations of the VariantAutoscaling's state |  | Optional: \{\} <br /> |


## Status Conditions

WVA uses standard Kubernetes conditions to report the health and state of each VariantAutoscaling resource.

### Condition Types

#### MetricsAvailable

Indicates whether vLLM metrics are available from Prometheus for this variant.

**Status Values:**
- `True`: Metrics data is available
- `False`: No metrics available (pods not ready or metrics not scraped)

**Common Reasons:**
- `MetricsAvailable`: Saturation metrics data is available for scaling decisions
- `MetricsUnavailable`: No saturation metrics available

**Example:**
```yaml
conditions:
- type: MetricsAvailable
  status: "True"
  reason: MetricsAvailable
  message: "Saturation metrics data is available for scaling decisions"
  lastTransitionTime: "2026-01-09T22:00:00Z"
```

#### OptimizationReady

Indicates whether the optimization engine ran successfully.

**Status Values:**
- `True`: Optimization completed
- `False`: Optimization failed or cannot run

**Common Reasons:**
- `OptimizationSucceeded`: Optimization completed successfully
- `SaturationOnlyMode`: Operating in saturation-only mode
- `SaturationSafetyOverride`: Saturation safety override applied

**Example:**
```yaml
conditions:
- type: OptimizationReady
  status: "True"
  reason: OptimizationSucceeded
  message: "Hybrid mode: scale-up decision (target: 3 replicas)"
  lastTransitionTime: "2026-01-09T22:00:00Z"
```

### Viewing Conditions

```bash
# Quick view with MetricsReady column
kubectl get variantautoscaling -A

# Detailed condition information
kubectl describe variantautoscaling <name> -n <namespace>

# Extract conditions as JSON
kubectl get variantautoscaling <name> -n <namespace> -o jsonpath='{.status.conditions}' | jq
```

For comprehensive information on metrics health monitoring and troubleshooting, see [Metrics Health Monitoring](../metrics-health-monitoring.md).


