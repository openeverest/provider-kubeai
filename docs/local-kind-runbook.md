# Local CPU/KIND runbook

This runbook validates the complete `Instance -> KubeAI Model -> OpenAI-compatible request` path using an Ollama model on CPU. It deliberately installs KubeAI and the OpenEverest `Instance` in `default`: KubeAI serves `Model` resources from its own namespace, and this provider creates the `Model` in the same namespace as the `Instance`.

## Prerequisites

Run every command from the repository root.

```sh
go version                  # Must select Go 1.26.2.
docker version
helm version
kubectl version --client
kind version                # Required for the KIND flow below.
```

The module pins OpenEverest `v2.0.0-dev.1`; use CRDs from that selected module
version rather than copying paths or manifests from another release.

The repository tracks the static Helm chart:

```text
charts/provider-kubeai/
  Chart.yaml
  values.yaml
  templates/
    _helpers.tpl
    provider.yaml
    clusterrole.yaml
    clusterrolebinding.yaml
    deployment.yaml
    serviceaccount.yaml
    service.yaml
```

## Generated file ownership

| Path | Owner | Edit policy |
| --- | --- | --- |
| `config/rbac/role.yaml` | `make manifests` | Never edit; regenerate from Kubebuilder markers. |
| `charts/provider-kubeai/generated/provider-spec.yaml` | `go generate ./...` / provider SDK | Never edit; regenerate from `definition/`. |
| `charts/provider-kubeai/generated/rbac-rules.yaml` | `make helm-sync-rbac` | Never edit; regenerate from `internal/provider/rbac.go`. |
| `charts/provider-kubeai/Chart.yaml`, `values.yaml`, and `templates/` | Repository | Edit for provider-specific chart behavior. |

## Step-by-step development flow

1. Inspect available targets:

   ```sh
   make help
   ```

2. Generate all derived files after changing `definition/` or RBAC markers:

   ```sh
   make generate
   test -s config/rbac/role.yaml
   test -s charts/provider-kubeai/generated/rbac-rules.yaml
   test -s charts/provider-kubeai/generated/provider-spec.yaml
   ```

   `make generate` installs pinned generator tools as necessary, generates
   `config/rbac/role.yaml`, copies its `rules:` mapping to the Helm chart, and
   generates the Provider spec. Never manually edit either file in
   `charts/provider-kubeai/generated/`.

3. Compile and test the provider:

   ```sh
   go build ./...
   make test
   make lint
   ```

4. Render and lint the complete static and generated Helm chart:

   ```sh
   make helm-template > /tmp/provider-kubeai-rendered.yaml
   helm lint charts/provider-kubeai
   ```

5. Check that all generated output is committed and reproducible:

   ```sh
   make verify
   ```

   This runs generation and fails if it changes `config/` or
   `charts/provider-kubeai/generated/`.

`go generate ./...` reads `definition/provider.yaml`, `definition/versions.yaml`,
the Go definition types, and topology definitions. Fix generation failures in
`definition/`; do not edit the generated Provider spec to silence them.

## Make target reference

| Target | Command and purpose |
| --- | --- |
| `help` | `make help` lists all targets. |
| `run` | `make run` regenerates artifacts, then runs the provider against the current Kubernetes context. |
| `lint` | `make lint` installs the pinned `golangci-lint` binary if needed and lints the Go module. |
| `test` | `make test` runs unit tests and writes coverage to `cover.out`. |
| `manifests` | `make manifests` generates `config/rbac/role.yaml` from Kubebuilder RBAC markers. |
| `helm-sync-rbac` | `make helm-sync-rbac` copies the RBAC `rules:` mapping to `charts/provider-kubeai/generated/rbac-rules.yaml`. |
| `generate` | `make generate` runs `manifests`, `helm-sync-rbac`, and the Provider SDK generator. Run it after definition or RBAC changes. |
| `verify` | `make verify` reruns generation and fails when generated files differ from Git. Intended for CI. |
| `build` | `make build` regenerates artifacts and writes the provider executable to `bin/provider`. |
| `docker-build` | `make docker-build` builds `ghcr.io/openeverest/provider-kubeai-dev:latest`; override with `make docker-build IMG=repository/image:tag`. |
| `docker-push` | `make docker-push` pushes `IMG`; build and authenticate first. |
| `helm-template` | `make helm-template` renders the local chart without installing it. |
| `helm-install` | `make helm-install` installs release `provider-kubeai` from the local chart into Helm's current/default namespace. |
| `helm-upgrade` | `make helm-upgrade` upgrades existing release `provider-kubeai` from the local chart. |
| `helm-uninstall` | `make helm-uninstall` removes release `provider-kubeai`. |
| `test-integration` | `make test-integration` reports that KUTTL test assets have not been restored yet. |
| `k3d-cluster-up` | `make k3d-cluster-up` creates the development k3d cluster from `dev/k3d.yaml`. |
| `k3d-cluster-down` | `make k3d-cluster-down` deletes that k3d cluster. |
| `k3d-cluster-reset` | `make k3d-cluster-reset` deletes, then recreates, the development k3d cluster. |
| `controller-gen` | `make controller-gen` installs the pinned controller-gen tool in `bin/`. |
| `yq` | `make yq` installs the pinned yq tool in `bin/`. |
| `golangci-lint` | `make golangci-lint` installs the pinned linter in `bin/`. |

## Create the CPU example

`examples/instance-simple.yaml` is the checked-in CPU-only example. Its intent is:

- namespace: `default`;
- instance name: `qwen2-05b-cpu`;
- server model source: `ollama://qwen2:0.5b`;
- no GPU limit;
- resource profile: `cpu:1` (explicit or derived by the provider);
- autoscaled topology with `minReplicas: 1` and `maxReplicas: 1`.

The provider maps `ollama://` sources to the `OLlama` KubeAI engine. The
example explicitly selects `resourceProfile: cpu:1`.

Keep a separate GPU example only for a GPU-capable cluster. It must request a GPU resource and use an `hf://` source so the provider selects the VLLM engine.

## Run the end-to-end test

### 1. Create the KIND cluster

```sh
kind create cluster --name kubeai-test
kubectl config use-context kind-kubeai-test
kubectl get nodes
```

On Docker Desktop, give the VM enough CPU and memory for a model pod. The 0.5B model is intentionally small, but its first download and CPU inference are still slow.

### 2. Install the OpenEverest CRDs matching this checkout

Do not hard-code the OpenEverest version in this command. Resolve the directory from the selected module:

```sh
OE_DIR="$(go list -m -f '{{.Dir}}' github.com/openeverest/openeverest/v2)"
kubectl apply -f "$OE_DIR/config/crd/bases/"
kubectl get crd | rg openeverest

# Validate the rendered chart without creating resources.
kubectl apply --dry-run=client -f /tmp/provider-kubeai-rendered.yaml
```

If this module version does not contain `config/crd/bases`, obtain the CRDs from the corresponding OpenEverest release or development checkout instead. The provider cannot reconcile `Instance` resources until those CRDs are installed.

### 3. Install KubeAI in `default`

```sh
helm repo add kubeai https://www.kubeai.org
helm repo update
helm install kubeai kubeai/kubeai \
  --namespace default \
  --wait --timeout 10m

kubectl get deploy,svc -n default
kubectl get crd models.kubeai.org
```

Do not install KubeAI in a separate `kubeai` namespace for this test unless the provider is changed to create Models there too. The provider uses the Instance namespace when applying the Model.

### 4. Register the Provider custom resource

```sh
helm template provider-kubeai charts/provider-kubeai \
  --namespace default \
  --show-only templates/provider.yaml \
  | kubectl apply -f -

kubectl get providers
```

### 5. Run the provider on the Mac

In terminal one:

```sh
make run
```

The provider process uses the current Kubernetes context, so it must remain `kind-kubeai-test`. This fast loop does not build a provider image or install the provider chart.

In terminal two:

```sh
kubectl apply -f examples/instance-simple.yaml
kubectl get instances -n default -w
```

Expected progression is `Provisioning`, then `Initializing` while KubeAI downloads the model, then `Ready` once at least one Model replica is ready.

## Verify every reconciliation responsibility

```sh
# Sync created one Model with the expected source, engine, and replica bounds.
kubectl get models -n default
kubectl get model qwen2-05b-cpu -n default -o yaml

# KubeAI starts the backing pod. The first start downloads the Ollama artifact.
kubectl get pods -n default -w

# Inspect the provider's status connection information.
kubectl get secret qwen2-05b-cpu-conn -n default -o yaml

# Inspect failures or slow pulls.
kubectl logs deployment/kubeai -n default
kubectl logs -n default -l kubeai.org/model=qwen2-05b-cpu --all-containers=true
```

The exact labels on a model pod can vary by KubeAI version. If the last `kubectl logs` command selects no pod, get the pod name from `kubectl get pods -n default` and use `kubectl logs POD_NAME -n default --all-containers=true`.

Test validation independently by applying a copy of the CPU Instance whose model source has an unsupported scheme, for example `https://example.invalid/model`. The provider's `Validate` method must reject it with the supported-source-scheme error. Do not change the working CPU Instance in place while it is the subject of the readiness test.

Test owner-reference cleanup after the ready and API tests:

```sh
kubectl delete instance qwen2-05b-cpu -n default
kubectl get model qwen2-05b-cpu -n default -w
```

The Model should be garbage-collected because `Sync()` applies it with the Instance as owner. Its serving pod should then disappear as KubeAI processes Model deletion.

## Call the local OpenAI-compatible API

Once the Instance is Ready:

```sh
kubectl port-forward --namespace default svc/kubeai 8000:80
```

In another terminal:

```sh
curl http://localhost:8000/openai/v1/models

curl http://localhost:8000/openai/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"qwen2-05b-cpu","messages":[{"role":"user","content":"Say hi in one sentence"}]}'
```

The model value is the `Instance` name because the provider creates a KubeAI `Model` with that same name.

## Test the in-cluster provider image

Use this only after the fast local loop succeeds:

```sh
make docker-build IMG=provider-kubeai:dev
kind load docker-image provider-kubeai:dev --name kubeai-test

helm upgrade --install provider-kubeai charts/provider-kubeai \
  --namespace default \
  --set image.repository=provider-kubeai \
  --set image.tag=dev \
  --set image.pullPolicy=Never \
  --wait --timeout 5m

kubectl get pods -n default
kubectl logs -n default deployment/provider-kubeai -f
```

Then repeat the CPU Instance apply, status, Model, API, validation, and cleanup checks above.

## Cleanup

```sh
helm uninstall provider-kubeai -n default 2>/dev/null || true
helm uninstall kubeai -n default 2>/dev/null || true
kind delete cluster --name kubeai-test
```

If using the local `make run` loop, stop it with `Ctrl-C` before deleting the cluster.
