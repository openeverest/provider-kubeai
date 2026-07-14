# GPU runbook

End-to-end path for a **vLLM GPU** Instance on a real NVIDIA cluster:

`Instance → provider Sync → KubeAI Model (VLLM) → GPU Pod → /openai/v1`

This does **not** run on Mac KIND/k3d (no CUDA). Use [local-kind-runbook.md](./local-kind-runbook.md) or [local-k3d-runbook.md](./local-k3d-runbook.md) for the CPU/Ollama path.

## Manifests in this repo

| Path | Role |
| --- | --- |
| `examples/instance-gpu.yaml` | Sample GPU Instance (`hf://` + `nvidia-gpu-l4:1`) |
| `examples/instance-simple.yaml` | CPU/Ollama Instance (local only) |
| `deploy/kubeai/values-gpu.yaml` | KubeAI Helm overrides (resource profiles + PodMonitor) |
| `deploy/kubeai/hf-token-secret.yaml` | Template Secret for gated Hugging Face models |
| `deploy/observability/values-prometheus.yaml` | Optional Prometheus Operator scrape config |
| `examples/observability/vllm-grafana-dashboard.json` | KubeAI vLLM Grafana dashboard |
| `docs/observability.md` | Full Prometheus + Grafana setup |
| `charts/provider-kubeai/` | Same provider chart for CPU and GPU |

## Prerequisites

- Kubernetes cluster with allocatable `nvidia.com/gpu` (GKE/EKS/AKS/on-prem + NVIDIA GPU Operator or cloud GPU addon).
- `helm`, `kubectl`, and a kubeconfig pointing at that cluster.
- OpenEverest `Instance` / `Provider` CRDs installed (same pin as local runbook).
- KubeAI and the Instance in the **same namespace** (this guide uses `default`).

Verify GPUs:

```sh
kubectl get nodes -o custom-columns=NAME:.metadata.name,GPU:.status.allocatable.nvidia\\.com/gpu
```

At least one node should show a GPU count ≥ 1.

## 1. Install KubeAI (GPU values)

```sh
helm repo add kubeai https://www.kubeai.org
helm repo update

# Edit deploy/kubeai/values-gpu.yaml nodeSelector/tolerations for your cloud first.
helm upgrade --install kubeai kubeai/kubeai -n default \
  -f deploy/kubeai/values-gpu.yaml \
  --wait --timeout 10m

kubectl get deploy,svc -n default -l app.kubernetes.io/name=kubeai
```

Confirm the profile name you will use exists in the installed values (e.g. `nvidia-gpu-l4`). The Instance uses `resourceProfile: nvidia-gpu-l4:1` (name + count).

## 2. Hugging Face token (gated models)

```sh
# Edit stringData.HF_TOKEN, then:
kubectl apply -f deploy/kubeai/hf-token-secret.yaml
```

Ensure model pods receive `HF_TOKEN` (KubeAI model `env` / `envFrom`, or a future provider Sync field). Without it, gated pulls such as `meta-llama/*` fail.

For a public small GPU smoke test, change `examples/instance-gpu.yaml` `model.source` to a public HF repo before applying.

## 3. Install / run the provider

In-cluster:

```sh
helm upgrade --install provider-kubeai charts/provider-kubeai -n default
```

Or locally against the cluster (debug):

```sh
make generate
go run cmd/provider/main.go
```

Confirm the Provider CR exists:

```sh
kubectl get providers.core.openeverest.io
```

## 4. Apply the GPU Instance

```sh
kubectl apply -f examples/instance-gpu.yaml

kubectl get instance llama-3-8b -w
kubectl get model llama-3-8b -o yaml
kubectl get pods -l model=llama-3-8b -w
```

Expected Model fields:

- `spec.engine: VLLM`
- `spec.url: hf://...`
- `spec.resourceProfile: nvidia-gpu-l4:1`
- `spec.minReplicas` / `maxReplicas` from topology

Provider must **not** thrash `spec.replicas` (Sync uses CreateOrUpdate and leaves replicas to KubeAI).

## 5. Call the API

```sh
kubectl port-forward -n default svc/kubeai 8000:80
```

```sh
curl -s http://127.0.0.1:8000/openai/v1/models | jq

curl --max-time 600 http://127.0.0.1:8000/openai/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "llama-3-8b",
    "messages": [{"role": "user", "content": "Say hi in one sentence"}],
    "max_tokens": 32
  }' | jq
```

First request with `minReplicas: 0` pays cold start (schedule + image + weight download). Use `/openai/v1/...` (not `/v1/...`).

## 6. Scale-to-zero check

With no traffic, after `scaleDownDelaySeconds`, the model pod should go away and Instance phase may become `Suspended`. A new chat request should scale back up.

```sh
kubectl get model llama-3-8b -o jsonpath='{.spec.replicas} {.status.replicas}{"\n"}'
kubectl get pods -l model=llama-3-8b
```

## 7. Observability (TTFT / ITL)

Full steps (Prometheus stack, PodMonitor, Grafana import): [observability.md](./observability.md).

```sh
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm upgrade --install prometheus prometheus-community/kube-prometheus-stack \
  -n monitoring --create-namespace \
  -f deploy/observability/values-prometheus.yaml
```

KubeAI `values-gpu.yaml` already sets `metrics.prometheusOperator.vLLMPodMonitor.enabled: true`.

Key vLLM metrics (after GPU traffic):

- `vllm:time_to_first_token_seconds` — TTFT
- `vllm:inter_token_latency_seconds` — ITL
- `vllm:e2e_request_latency_seconds`
- `vllm:kv_cache_usage_perc`

Import `examples/observability/vllm-grafana-dashboard.json` into Grafana (port-forward `svc/prometheus-grafana` in `monitoring`).

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Pod `Pending` | Node GPUs, profile `nodeSelector`/`tolerations`, `kubectl describe pod` |
| Weight pull fails | `HF_TOKEN`, model id, network to Hugging Face |
| `404` on curl | Use `/openai/v1/...`; port-forward `svc/kubeai` |
| Model generation skyrockets | Provider Sync must not full-Update clearing `replicas` |
| Empty `/openai/v1/models` | Instance/Model in same ns as KubeAI; Model exists |

## CPU vs GPU summary

| | Local KIND / k3d | GPU cluster |
| --- | --- | --- |
| Example | `examples/instance-simple.yaml` | `examples/instance-gpu.yaml` |
| Engine | OLlama | VLLM |
| Profile | `cpu:1` | `nvidia-gpu-l4:1` (or your profile) |
| KubeAI values | chart defaults | `deploy/kubeai/values-gpu.yaml` |
| Docs | [local-kind-runbook.md](./local-kind-runbook.md), [local-k3d-runbook.md](./local-k3d-runbook.md) | this file |
| TTFT/ITL | not the production signal | vLLM `/metrics` + PodMonitor |
