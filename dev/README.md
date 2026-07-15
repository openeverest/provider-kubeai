# Provider development with Tilt

This directory contains a [Tilt](https://tilt.dev/) setup for developing
`provider-kubeai`. It installs the latest released OpenEverest v2 core and the
[KubeAI](https://www.kubeai.org/) operator, then builds and deploys this
provider, with live-reload on every code change.

You do **not** need a local checkout of the OpenEverest core.

## Prerequisites

- Docker
- kubectl
- Helm
- [k3d](https://k3d.io/)
- [Tilt](https://docs.tilt.dev/install.html)

## Quick start

```sh
# 1. (Optional) configure the environment
cp dev/.env.example dev/.env

# 2. Create the local cluster and start Tilt
make dev-up
```

Tilt opens its dashboard at <http://localhost:10350>. Once everything is green:

- The Everest UI/API is available at <http://localhost:8080>
  (default credentials: `admin` / `admin`).
- The KubeAI OpenAI-compatible API is port-forwarded to <http://localhost:8000>.

Apply an example Instance to exercise the provider (CPU/Ollama, safe on a
laptop):

```sh
kubectl apply -f examples/instance-simple.yaml
kubectl get instances -w
```

Wake the model and call it (use `/openai/v1/...`, not `/v1/...`):

```sh
curl http://127.0.0.1:8000/openai/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"qwen2-05b-cpu","messages":[{"role":"user","content":"hi"}],"max_tokens":32}'
```

Edit any provider Go code and Tilt rebuilds the binary and live-updates the
running pod without recreating it.

To tear things down:

```sh
make dev-down      # stop Tilt (keeps the cluster)
make dev-destroy   # stop Tilt and delete the cluster
```

## Configuration

All settings live in `dev/.env` (see `dev/.env.example`). Common options:

| Variable | Default | Description |
| --- | --- | --- |
| `INSTALL_OPENEVEREST` | `true` | Install the released OpenEverest core. |
| `OPENEVEREST_VERSION` | `2.0.0-dev.1` | Pin the core chart version (required while v2 is pre-release). |
| `INSTALL_KUBEAI` | `true` | Install the KubeAI operator. |
| `KUBEAI_VERSION` | _(latest)_ | Pin the KubeAI chart version. |
| `KUBEAI_EXTRA_VALUES` | _(none)_ | Extra KubeAI values file, e.g. `deploy/kubeai/values-gpu.yaml`. |
| `PROVIDER_NAMESPACE` | `default` | Namespace for KubeAI, the provider, and Instances. |

> **Note:** While OpenEverest v2 is in pre-release, the Helm repository only
> publishes pre-release tags (e.g. `2.0.0-dev.1`). Helm's "latest" resolution
> skips pre-releases, so `OPENEVEREST_VERSION` must be set explicitly until
> v2.0.0 is generally available. It defaults to the version this provider is
> built against.

> **Namespace constraint:** KubeAI only serves `Model` resources from its own
> namespace, and this provider creates the Model in the same namespace as the
> Instance. Keep KubeAI, the provider, and your Instances in
> `PROVIDER_NAMESPACE` (default `default`).

## GPU profiles

The default KubeAI install is CPU-friendly for local dev. To exercise GPU
resource profiles (only meaningful on a cluster with `nvidia.com/gpu`), point
KubeAI at the GPU values file:

```sh
# dev/.env
KUBEAI_EXTRA_VALUES=deploy/kubeai/values-gpu.yaml
```

k3d on a laptop has no GPU — use this against a real GPU cluster via
`K8S_CONTEXT`. See `docs/gpu-runbook.md`.

## Developing the core and the provider together

When you need to test against a locally built core (not a release), run two
Tilt instances against the same cluster:

1. In the OpenEverest core repo, start the core dev environment (`make dev-up`).
   It manages `everest-system` and the core CRDs.

2. In this repo, start the provider Tilt instance on a different port with the
   core installation disabled:

   ```sh
   INSTALL_OPENEVEREST=false tilt up -f dev/Tiltfile --port 10351
   ```

The two instances manage disjoint Kubernetes objects, so they run side by side
without conflicting. With `INSTALL_OPENEVEREST=false`, the OpenEverest core CRDs
are expected to already exist in the cluster (installed by the core Tilt
instance). Set `INSTALL_KUBEAI=false` too if KubeAI is managed elsewhere.
