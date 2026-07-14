# Local CPU/k3d runbook

Same end-to-end path as the KIND guide — `Instance → KubeAI Model → OpenAI-compatible API` with a small Ollama model on CPU — using the checked-in k3d config and Make targets.

For KIND instead, see [local-kind-runbook.md](./local-kind-runbook.md). GPU/vLLM needs a real NVIDIA cluster ([gpu-runbook.md](./gpu-runbook.md)).

## What this repo already provides

| Path / target | Role |
| --- | --- |
| `dev/k3d.yaml` | k3d cluster config (`provider-kubeai-test`, local registry `:5000`) |
| `make k3d-cluster-up` | Create the cluster |
| `make k3d-cluster-down` | Delete the cluster |
| `make k3d-cluster-reset` | Delete + recreate |
| `examples/instance-simple.yaml` | CPU Ollama Instance (`qwen2-05b-cpu`) |

Cluster layout from `dev/k3d.yaml`:

- 1 server + 3 agents (k3s `v1.33.2-k3s1`)
- Embedded registry `k3d-registry` on host port `5000`
- Traefik, metrics-server, and servicelb disabled
- Load balancer disabled — use `kubectl port-forward` for APIs

## Prerequisites

Run every command from the repository root.

```sh
go version                  # Must select Go 1.26.2.
docker version
helm version
kubectl version --client
k3d version
```

Install k3d if needed: https://k3d.io/

Use OpenEverest CRDs from the module version pinned in `go.mod` (same as the KIND runbook).

## 1. Create the k3d cluster

```sh
make k3d-cluster-up
```

Equivalent:

```sh
k3d cluster create --config ./dev/k3d.yaml
```

Confirm context and nodes:

```sh
kubectl config use-context k3d-provider-kubeai-test
kubectl get nodes
```

Reset later with:

```sh
make k3d-cluster-reset   # or: make k3d-cluster-down && make k3d-cluster-up
```

Give Docker enough CPU/memory for the 0.5B Ollama pull and first inference.

## 2. Install OpenEverest CRDs

```sh
OE_DIR="$(go list -m -f '{{.Dir}}' github.com/openeverest/openeverest/v2)"
kubectl apply -f "$OE_DIR/config/crd/bases/"
kubectl get crd | rg openeverest
```

If this module version has no `config/crd/bases`, use the matching OpenEverest release or checkout instead.

## 3. Install KubeAI in `default`

KubeAI and the Instance must share a namespace (examples use `default`).

```sh
helm repo add kubeai https://www.kubeai.org
helm repo update
helm upgrade --install kubeai kubeai/kubeai \
  --namespace default \
  --wait --timeout 10m

kubectl get deploy,svc -n default
kubectl get crd models.kubeai.org
```

## 4. Register the Provider CR

```sh
make generate
helm template provider-kubeai charts/provider-kubeai \
  --namespace default \
  --show-only templates/provider.yaml \
  | kubectl apply -f -

kubectl get providers
```

## 5. Run the provider (fast local loop)

Terminal one — process uses the current kubeconfig (`k3d-provider-kubeai-test`):

```sh
make run
```

Terminal two:

```sh
kubectl apply -f examples/instance-simple.yaml
kubectl get instances -n default -w
```

Expected: `Provisioning` → `Initializing` (model download) → `Ready`.

## 6. Verify reconciliation

```sh
kubectl get models -n default
kubectl get model qwen2-05b-cpu -n default -o yaml
kubectl get pods -n default -w

kubectl get secret qwen2-05b-cpu-conn -n default -o yaml
kubectl logs deployment/kubeai -n default
kubectl logs -n default -l kubeai.org/model=qwen2-05b-cpu --all-containers=true
```

If the label selector finds no pods, pick the pod name from `kubectl get pods -n default` and log that pod.

Owner cleanup:

```sh
kubectl delete instance qwen2-05b-cpu -n default
kubectl get model qwen2-05b-cpu -n default -w
```

## 7. Call the OpenAI-compatible API

```sh
kubectl port-forward --namespace default svc/kubeai 8000:80
```

```sh
curl http://localhost:8000/openai/v1/models

curl http://localhost:8000/openai/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"qwen2-05b-cpu","messages":[{"role":"user","content":"Say hi in one sentence"}]}'
```

Use `/openai/v1/...` (not `/v1/...`). The model name is the Instance name.

## 8. Optional: in-cluster provider image

After the local `make run` loop works:

```sh
make docker-build IMG=provider-kubeai:dev
k3d image import provider-kubeai:dev -c provider-kubeai-test

helm upgrade --install provider-kubeai charts/provider-kubeai \
  --namespace default \
  --set image.repository=provider-kubeai \
  --set image.tag=dev \
  --set image.pullPolicy=Never \
  --wait --timeout 5m

kubectl get pods -n default
kubectl logs -n default deployment/provider-kubeai -f
```

Then re-apply `examples/instance-simple.yaml` and repeat the API checks.

The cluster also creates a registry on `localhost:5000` (`k3d-registry`). You can push there instead of `k3d image import` if you prefer tag/push workflows.

## Cleanup

```sh
helm uninstall provider-kubeai -n default 2>/dev/null || true
helm uninstall kubeai -n default 2>/dev/null || true
make k3d-cluster-down
```

Stop a local `make run` with `Ctrl-C` before deleting the cluster.

## KIND vs k3d (this repo)

| | KIND | k3d |
| --- | --- | --- |
| Config | ad-hoc `kind create` | `dev/k3d.yaml` + Make |
| Cluster name | `kubeai-test` | `provider-kubeai-test` |
| Context | `kind-kubeai-test` | `k3d-provider-kubeai-test` |
| Docs | [local-kind-runbook.md](./local-kind-runbook.md) | this file |
| Image load | `kind load docker-image ...` | `k3d image import ... -c provider-kubeai-test` |
| Same Instance YAML | yes | yes |

Both are valid for CPU/Ollama. Prefer k3d when you want the Make targets and embedded registry; prefer KIND when following the existing KIND-first quick start in the README.
