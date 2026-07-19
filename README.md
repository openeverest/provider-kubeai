# KubeAI Provider

OpenEverest provider that turns `Instance` CRs into [KubeAI](https://www.kubeai.org/) `Model` CRs for LLM serving (vLLM on GPU, Ollama on CPU for local demos).

## Installation

The provider chart is published as an OCI artifact to the GitHub Container
Registry. [KubeAI](https://www.kubeai.org/) itself is not bundled — install it
separately (see the Quick Start below) before or alongside the provider.

```bash
helm install provider-kubeai \
  oci://ghcr.io/openeverest/charts/provider-kubeai \
  --version 0.1.0 \
  --create-namespace
```

Upgrade to a newer chart version:

```bash
helm upgrade provider-kubeai \
  oci://ghcr.io/openeverest/charts/provider-kubeai \
  --version 0.1.0
```

Uninstall:

```bash
helm uninstall provider-kubeai
```

> Browse available versions on the
> [chart package page](https://github.com/openeverest/provider-kubeai/pkgs/container/charts%2Fprovider-kubeai).

## Quick Start

### Prerequisites

- Go (see `go.mod`), Docker, Helm, kubectl
- A Kubernetes cluster with OpenEverest `Instance` / `Provider` CRDs installed
- [KubeAI](https://www.kubeai.org/) installed in the **same namespace** as your Instances (this repo’s examples use `default`)

Local CPU cluster — **KIND** ([docs/local-kind-runbook.md](docs/local-kind-runbook.md)):

```sh
kind create cluster --name kubeai-test
# Install OpenEverest CRDs from the pinned module version (see the KIND runbook)
helm repo add kubeai https://www.kubeai.org && helm repo update
helm upgrade --install kubeai kubeai/kubeai -n default --wait --timeout 10m
```

Or **k3d** with the checked-in config ([docs/local-k3d-runbook.md](docs/local-k3d-runbook.md)):

```sh
make k3d-cluster-up
kubectl config use-context k3d-provider-kubeai-test
# Install OpenEverest CRDs, then KubeAI (same as KIND runbook)
helm repo add kubeai https://www.kubeai.org && helm repo update
helm upgrade --install kubeai kubeai/kubeai -n default --wait --timeout 10m
```

Generate Provider CR manifests (after changing `definition/` or RBAC):

```sh
make generate
```

### Run the Provider

```sh
make run
```

Or deploy with Helm:

```sh
helm upgrade --install provider-kubeai charts/provider-kubeai -n default
```

### Create an Instance

CPU / local (Ollama):

```sh
kubectl apply -f examples/instance-simple.yaml
kubectl get instance
kubectl get model
kubectl get pods -l model=qwen2-05b-cpu
```

GPU / vLLM (NVIDIA cluster required):

```sh
kubectl apply -f examples/instance-gpu.yaml
kubectl get instance
kubectl get model
kubectl get pods -l model=llama-3-8b
```

### Call the API

```sh
kubectl port-forward svc/kubeai 8000:80
curl -s http://127.0.0.1:8000/openai/v1/models | jq
curl http://127.0.0.1:8000/openai/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"qwen2-05b-cpu","messages":[{"role":"user","content":"hi"}],"max_tokens":32}'
```

Use `/openai/v1/...` (not `/v1/...`). For the GPU example, set `"model":"llama-3-8b"`.

### Observability (vLLM + Prometheus)

On a GPU cluster, scrape vLLM metrics with kube-prometheus-stack and the KubeAI PodMonitor:

```sh
helm upgrade --install prometheus prometheus-community/kube-prometheus-stack \
  -n monitoring --create-namespace \
  -f deploy/observability/values-prometheus.yaml
# KubeAI already enables the PodMonitor via deploy/kubeai/values-gpu.yaml
```

Import `examples/observability/vllm-grafana-dashboard.json` into Grafana. Full steps: [docs/observability.md](docs/observability.md).

## License

Apache-2.0
