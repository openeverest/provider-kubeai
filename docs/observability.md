# Observability with Prometheus Stack (vLLM)

Follows [KubeAI: Configure Observability with Prometheus Stack](https://www.kubeai.org/how-to/observability-with-prometheus-stack/).

KubeAI exposes a **vLLM PodMonitor** that scrapes each vLLM pod’s `/metrics`. With the Prometheus Operator stack installed, those metrics land in Prometheus and can be viewed in Grafana via the checked-in dashboard.

## Manifests in this repo

| Path | Role |
| --- | --- |
| `deploy/observability/values-prometheus.yaml` | kube-prometheus-stack values so PodMonitors are discovered without special labels |
| `deploy/kubeai/values-gpu.yaml` | Enables `metrics.prometheusOperator.vLLMPodMonitor` |
| `examples/observability/vllm-grafana-dashboard.json` | Upstream KubeAI vLLM Grafana dashboard |

GPU Instances must be running before vLLM metrics appear. See [gpu-runbook.md](./gpu-runbook.md).

## 1. Deploy Prometheus Operator (kube-prometheus-stack)

```sh
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

helm upgrade --install prometheus prometheus-community/kube-prometheus-stack \
  -n monitoring --create-namespace \
  -f deploy/observability/values-prometheus.yaml
```

The values file sets `*SelectorNilUsesHelmValues: false` so Prometheus scrapes PodMonitors (and related resources) created by KubeAI without requiring Helm release labels.

## 2. Enable KubeAI’s vLLM PodMonitor

`deploy/kubeai/values-gpu.yaml` already contains:

```yaml
metrics:
  prometheusOperator:
    vLLMPodMonitor:
      enabled: true
      labels: {}
```

Install or upgrade KubeAI with those values:

```sh
helm repo add kubeai https://www.kubeai.org
helm repo update

helm upgrade --install kubeai kubeai/kubeai -n default \
  -f deploy/kubeai/values-gpu.yaml \
  --wait --timeout 10m
```

If KubeAI is already installed and you only need to turn the PodMonitor on:

```sh
helm upgrade --reuse-values --install kubeai kubeai/kubeai \
  --set metrics.prometheusOperator.vLLMPodMonitor.enabled=true
```

Confirm the PodMonitor exists:

```sh
kubectl get podmonitor -A | rg -i vllm
```

## 3. Generate traffic (so metrics have data)

Apply a GPU Instance and hit the OpenAI-compatible API (see [gpu-runbook.md](./gpu-runbook.md)):

```sh
kubectl apply -f examples/instance-gpu.yaml
kubectl port-forward -n default svc/kubeai 8000:80
# then chat/completions against the model name
```

Useful vLLM metrics after traffic:

- `vllm:time_to_first_token_seconds` — TTFT
- `vllm:inter_token_latency_seconds` — ITL
- `vllm:e2e_request_latency_seconds`
- `vllm:kv_cache_usage_perc`

## 4. Import the vLLM Grafana dashboard

Port-forward Grafana (release name `prometheus` → service `prometheus-grafana`):

```sh
kubectl port-forward -n monitoring svc/prometheus-grafana 8081:80
```

Open http://localhost:8081.

Default login is `admin` / `prom-operator`. If that fails:

```sh
kubectl get secret -n monitoring prometheus-grafana \
  -o jsonpath="{.data.admin-password}" | base64 --decode ; echo
```

In Grafana: **Dashboards → Import → Upload JSON file**, then select:

```text
examples/observability/vllm-grafana-dashboard.json
```

Choose the Prometheus datasource created by kube-prometheus-stack and import.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| No PodMonitor | KubeAI installed with `vLLMPodMonitor.enabled=true`; `kubectl get podmonitor -A` |
| Prometheus has no vLLM series | GPU model pod running; traffic to `/openai/v1/...`; PodMonitor selector values above |
| Grafana empty panels | Datasource = Prometheus; time range includes recent requests; metric names use `vllm:` prefix |
| Wrong Grafana service | `kubectl get svc -n monitoring \| rg grafana` |
