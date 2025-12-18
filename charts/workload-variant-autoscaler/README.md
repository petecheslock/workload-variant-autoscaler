# workload-variant-autoscaler

![Version: 0.4.3](https://img.shields.io/badge/Version-0.4.3-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v0.4.3](https://img.shields.io/badge/AppVersion-v0.4.3-informational?style=flat-square)

Helm chart for Workload-Variant-Autoscaler (WVA) - GPU-aware autoscaler for LLM inference workloads

## Installation Modes

WVA supports three installation modes to enable flexible deployment architectures:

### Mode 1: `all` (Default)
Installs both the WVA controller and model-specific resources in a single installation. This is the traditional mode and is backward compatible with previous versions.

**Use case**: Single llm-d stack with one model.

```bash
helm install workload-variant-autoscaler ./workload-variant-autoscaler \
  -n workload-variant-autoscaler-system \
  --set installMode=all
```

### Mode 2: `controller-only`
Installs only the WVA controller without any model-specific resources. This enables a cluster-wide controller that can manage multiple models across different namespaces.

**Use case**: Install the controller once for the entire cluster, then deploy model-specific resources separately as needed.

```bash
# Install the controller once
helm install wva-controller ./workload-variant-autoscaler \
  -n workload-variant-autoscaler-system \
  --create-namespace \
  --set installMode=controller-only \
  --set wva.namespaceScoped=false
```

### Mode 3: `model-resources-only`
Installs only model-specific resources (VariantAutoscaling, HPA, Service, ServiceMonitor) without the controller. Use this mode to add new models when a cluster-wide controller is already running.

**Use case**: Deploy resources for additional models in different namespaces after the controller is installed.

```bash
# Deploy model resources for Model A in namespace-a
helm install model-a-resources ./workload-variant-autoscaler \
  -n namespace-a \
  --set installMode=model-resources-only \
  --set llmd.namespace=namespace-a \
  --set llmd.modelName=model-a \
  --set llmd.modelID="vendor/model-a"

# Deploy model resources for Model B in namespace-b
helm install model-b-resources ./workload-variant-autoscaler \
  -n namespace-b \
  --set installMode=model-resources-only \
  --set llmd.namespace=namespace-b \
  --set llmd.modelName=model-b \
  --set llmd.modelID="vendor/model-b"
```

## Multi-Model Architecture Example

For supporting multiple llm-d stacks with a single controller:

```bash
# Step 1: Install the WVA controller once (cluster-wide)
helm install wva-controller ./workload-variant-autoscaler \
  -n workload-variant-autoscaler-system \
  --create-namespace \
  --set installMode=controller-only \
  --set wva.namespaceScoped=false \
  --set wva.prometheus.baseURL="https://prometheus:9090"

# Step 2: Deploy Model A resources
helm install model-a ./workload-variant-autoscaler \
  --set installMode=model-resources-only \
  --set llmd.namespace=llm-d-model-a \
  --set llmd.modelName=ms-model-a-llm-d-modelservice \
  --set llmd.modelID="meta-llama/Llama-2-7b-hf" \
  --set va.accelerator=H100

# Step 3: Deploy Model B resources (in a different namespace)
helm install model-b ./workload-variant-autoscaler \
  --set installMode=model-resources-only \
  --set llmd.namespace=llm-d-model-b \
  --set llmd.modelName=ms-model-b-llm-d-modelservice \
  --set llmd.modelID="mistralai/Mistral-7B-v0.1" \
  --set va.accelerator=A100
```

### Important Configuration Notes

**Namespace Scoping:**
- When using `installMode=controller-only` for multi-model deployments, you must set `wva.namespaceScoped=false` to allow the controller to watch all namespaces.
- When using `installMode=all` (default), you can keep `wva.namespaceScoped=true` for single-namespace operation or set it to `false` for cluster-wide operation.
- `installMode=model-resources-only` does not use the `namespaceScoped` setting since it doesn't install the controller.

**Resource Isolation:**
- Each model's resources (VariantAutoscaling, HPA, Service, ServiceMonitor) are deployed in the model's namespace.
- The controller remains in its dedicated namespace (typically `workload-variant-autoscaler-system`).
- Multiple Helm releases can coexist: one for the controller and one per model.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| installMode | string | `"all"` | Installation mode: "all" (controller + model resources), "controller-only" (just controller), "model-resources-only" (just model resources) |
| hpa.enabled | bool | `true` |  |
| hpa.maxReplicas | int | `10` |  |
| hpa.targetAverageValue | string | `"1"` |  |
| llmd.modelID | string | `"unsloth/Meta-Llama-3.1-8B"` |  |
| llmd.modelName | string | `"ms-inference-scheduling-llm-d-modelservice"` |  |
| llmd.namespace | string | `"llm-d-autoscaler"` |  |
| va.accelerator | string | `"H100"` |  |
| va.enabled | bool | `true` |  |
| va.sloTpot | int | `10` |  |
| va.sloTtft | int | `1000` |  |
| vllmService.enabled | bool | `true` |  |
| vllmService.interval | string | `"15s"` |  |
| vllmService.nodePort | int | `30000` |  |
| vllmService.scheme | string | `"http"` |  |
| wva.enabled | bool | `true` |  |
| wva.experimentalHybridOptimization | enum | `off` | supports on, off, and model-only |
| wva.namespaceScoped | bool | `true` | If true, controller watches only its namespace; if false, watches all namespaces (cluster-scoped) |
| wva.image.repository | string | `"ghcr.io/llm-d-incubation/workload-variant-autoscaler"` |  |
| wva.image.tag | string | `"latest"` |  |
| wva.imagePullPolicy | string | `"Always"` |  |
| wva.metrics.enabled | bool | `true` |  |
| wva.metrics.port | int | `8443` |  |
| wva.metrics.secure | bool | `true` |  |
| wva.prometheus.baseURL | string | `"https://thanos-querier.openshift-monitoring.svc.cluster.local:9091"` |  |
| wva.prometheus.monitoringNamespace | string | `"openshift-user-workload-monitoring"` |  |
| wva.prometheus.tls.caCertPath | string | `"/etc/ssl/certs/prometheus-ca.crt"` |  |
| wva.prometheus.tls.insecureSkipVerify | bool | `true` |  |
| wva.reconcileInterval | string | `"60s"` |  |
| wva.scaleToZero | bool | `false` |  |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.14.2](https://github.com/norwoodj/helm-docs/releases/v1.14.2)

### INSTALL (on OpenShift)

> **Note**: The default installation mode is `all`, which installs both the controller and model resources. For multi-model deployments, use `controller-only` mode first, then use `model-resources-only` mode for each model. See the [Installation Modes](#installation-modes) section above for details.

1. Before running, be sure to delete all previous helm installations for workload-variant-scheduler and prometheus-adapter.
2. llm-d must be installed for WVA to do it's magic. If you plan on installing llm-d with these instructions, please be sure to remove any other helm installation of llm-d before proceeding.

#### NOTE: to view which helm charts you already have installed in your cluster, use:
```
helm ls -A
```

```
export OWNER="llm-d-incubation"
export WVA_PROJECT="workload-variant-autoscaler"
export WVA_RELEASE="v0.4.1"
export WVA_NS="workload-variant-autoscaler-system"
export MON_NS="openshift-user-workload-monitoring"

kubectl get secret thanos-querier-tls -n openshift-monitoring -o jsonpath='{.data.tls\.crt}' | base64 -d > /tmp/prometheus-ca.crt

git clone -b $WVA_RELEASE -- https://github.com/$OWNER/$WVA_PROJECT.git $WVA_PROJECT
cd $WVA_PROJECT
export WVA_PROJECT=$PWD
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

helm upgrade -i prometheus-adapter prometheus-community/prometheus-adapter \
  -n $MON_NS \
  -f config/samples/prometheus-adapter-values-ocp.yaml

kubectl apply -f - <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: $WVA_NS
  labels:
    app.kubernetes.io/name: workload-variant-autoscaler
    control-plane: controller-manager
    openshift.io/user-monitoring: "true"
EOF

cd $WVA_PROJECT/charts
helm upgrade -i workload-variant-autoscaler ./workload-variant-autoscaler \
  -n $WVA_NS \
  --set-file wva.prometheus.caCert=/tmp/prometheus-ca.crt \
  --set va.accelerator=L40S \
  --set llmd.modelID=unsloth/Meta-Llama-3.1-8B \
  --set vllmService.enabled=true \
  --set vllmService.nodePort=30000
```

## Configuration Files

### Production vs Development Values

The Helm chart provides different configuration files for different environments:

#### Production Values (`values.yaml`)
- **TLS Verification**: Enabled (`insecureSkipVerify: false`)
- **Logging Level**: Production (`LOG_LEVEL: info`)
- **Security**: Strict security settings for production use
- **Saturation-based Scaling**: Conservative thresholds for production stability

#### Development Values (`values-dev.yaml`)
- **TLS Verification**: Relaxed (`insecureSkipVerify: true`) for easier development
- **Logging Level**: Debug (`LOG_LEVEL: debug`) for detailed development logging
- **Security**: Relaxed settings for development and testing
- **Saturation Scaling**: Aggressive thresholds for faster iteration

### Saturation Scaling Configuration

The chart includes saturation-based scaling thresholds that determine when replicas are saturated and when to scale up:

**Global Defaults** (applied to all models):
```yaml
wva:
  capacityScaling:
    default:
      kvCacheThreshold: 0.80      # Replica saturated if KV cache ≥ 80%
      queueLengthThreshold: 5     # Replica saturated if queue ≥ 5 requests
      kvSpareTrigger: 0.1         # Scale-up if spare KV capacity < 10%
      queueSpareTrigger: 3        # Scale-up if spare queue < 3
```

**Per-Model Overrides** (customize specific models):
```yaml
wva:
  capacityScaling:
    overrides:
      llm-d:
        modelID: "Qwen/Qwen3-0.6B"
        namespace: "llm-d-autoscaler"
        kvCacheThreshold: 0.70      # Lower threshold for production
        kvSpareTrigger: 0.35        # Avg spare KV <10% → scale-up
```

See `docs/saturation-scaling-config.md` for detailed configuration documentation.

### Usage Examples

#### Production Deployment
```bash
# Use production values (secure by default)
helm install workload-variant-autoscaler ./workload-variant-autoscaler \
  -n workload-variant-autoscaler-system \
  --values values.yaml
```

#### Development Deployment
```bash
# Use development values (relaxed security)
helm install workload-variant-autoscaler ./workload-variant-autoscaler \
  -n workload-variant-autoscaler-system \
  --values values-dev.yaml
```

#### Custom Configuration
```bash
# Override specific values
helm install workload-variant-autoscaler ./workload-variant-autoscaler \
  -n workload-variant-autoscaler-system \
  --values values.yaml \
  --set wva.prometheus.tls.insecureSkipVerify=true \
  --set wva.image.tag=v0.0.1-dev
```

### CLEANUP
```
export MON_NS="openshift-user-workload-monitoring"
export WVA_NS="workload-variant-autoscaler-system"

helm delete prometheus-adapter -n $MON_NS
helm delete workload-variant-autoscaler -n $WVA_NS
kubectl delete ns $WVA_NS
```

### VALIDATION / TROUBLESHOOTING
1. Check for 'error' in workload-variant-autoscaler-controller-manager-xxxxx in the workload-variant-autoscaler-system namespace
```
kubectl logs pod workload-variant-autoscaler-controller-manager-xxxxx -n workload-variant-autoscaler-system | grep error
```
2. Check for '404' in prometheus-adapter in the openshift-user-workload-monitoring namespace
```
kubectl logs pod prometheus-adapter-xxxxx -n openshift-user-workload-monitoring | grep 404
```
3. Check, after a few minutes following installation, for metric collection
```
kubectl get --raw "/apis/external.metrics.k8s.io/v1beta1/namespaces/$NAMESPACE/inferno_desired_replicas" | jq
```
