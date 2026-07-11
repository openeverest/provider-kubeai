# KubeAI Provider

OpenEverest provider that turns `Instance` CRs into [KubeAI](https://www.kubeai.org/) `Model` CRs for LLM serving (vLLM on GPU, Ollama on CPU for local demos).

## Quick Start

### Prerequisites

- Go (see `go.mod`), Docker, Helm, kubectl
- A Kubernetes cluster with OpenEverest `Instance` / `Provider` CRDs installed
- [KubeAI](https://www.kubeai.org/) installed in the **same namespace** as your Instances (this repo’s examples use `default`)

Local CPU cluster (KIND):

```sh
kind create cluster --name kubeai-test
# Install OpenEverest CRDs from the pinned module version (see docs/local-kind-runbook.md)
helm repo add kubeai https://www.kubeai.org && helm repo update
helm upgrade --install kubeai kubeai/kubeai -n default --wait --timeout 10m
```

GPU cluster: follow [docs/gpu-runbook.md](docs/gpu-runbook.md) and install KubeAI with:

```sh
helm upgrade --install kubeai kubeai/kubeai -n default \
  -f deploy/kubeai/values-gpu.yaml --wait --timeout 10m
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


## License

Apache-2.0
