# Model catalog

Lets an Admin curate a fixed list of LLMs at Helm install/upgrade time so they show up
in the Everest UI as a one-click dropdown, instead of users hand-writing
`spec.components.server.parameters.model.source` YAML.

## How it works

There is no `modelCatalog` field on the `Provider` CR. `Provider.spec` is a fixed
schema owned by the `openeverest` core repo (`ProviderSpec` in
`api/core/v1alpha1/provider_types.go`); Kubernetes prunes any field the CRD doesn't
declare, and the `provider-sdk generate` tool that builds
`charts/provider-kubeai/generated/provider-spec.yaml` only reads `name`, `components`,
and `parametersSchema` out of `definition/provider.yaml`. Adding an arbitrary
`modelCatalog` key there would silently go nowhere.

Instead, each catalog entry is rendered as a cluster-scoped `InstancePreset`
(`core.openeverest.io/v1alpha1`) — a CRD that already ships in the `openeverest` core
API this provider depends on. An `InstancePreset.spec` is just an `Instance.spec`
template (`providerRef`, `version`, `topology`, `components`, …), scoped to a provider
via `spec.providerRef.name`. The Everest UI/API already supports
`GET /instance-presets?provider=<name>` for listing them. This means:

- No change to `internal/provider/provider.go` — an Instance created from a preset
  looks exactly like a hand-written one, so `Validate`/`Sync` are untouched.
- No change to `definition/provider.yaml` or the generated Provider CR spec.
- No dependency on an upstream `openeverest` core release — the CRD is already
  vendored in this repo's `go.mod`.

## Admin: defining the catalog

Set `modelCatalog` in your Helm values (see the commented examples in
`charts/provider-kubeai/values.yaml`):

```yaml
modelCatalog:
  - name: qwen2-05b-cpu
    displayName: "Qwen2 0.5B (Ollama, CPU)"
    version: ollama-cpu          # must be a bundle name in definition/versions.yaml
    minReplicas: 1
    maxReplicas: 1
    model:
      source: ollama://qwen2:0.5b
    resourceProfile: cpu:1        # must exist in your KubeAI values (see values-gpu.yaml)
  - name: llama-3-8b-gpu
    displayName: "Llama 3.1 8B (vLLM, GPU)"
    version: vllm-0.11.2
    minReplicas: 0
    maxReplicas: 2
    task: TextGeneration
    model:
      source: hf://meta-llama/Llama-3.1-8B-Instruct
      estimatedParamBillions: 8
      quantization: fp16
    resourceProfile: nvidia-gpu-l4:1
    args:
      - --max-model-len=8192
```

Apply with:

```sh
helm upgrade --install provider-kubeai charts/provider-kubeai -n default -f my-values.yaml
```

`charts/provider-kubeai/templates/presets/instancepresets.yaml` renders one
`InstancePreset` per entry, named `<release>-<name>`. Changing the catalog is always
a `helm upgrade`; there is no separate catalog-apply step.

### Validation

The chart template fails at render time (before anything is applied) if:

- an entry is missing `name`, `version`, `model.source`, or `resourceProfile`,
- two entries share the same `name`, or
- `version` doesn't match a bundle name in `definition/versions.yaml`'s `versions:`
  list (`spec.versions` in `generated/provider-spec.yaml`).

`resourceProfile` is **not** chart-validated against your KubeAI installation's
`resourceProfiles` — that lives in a separate Helm release
(`deploy/kubeai/values-gpu.yaml`). If a catalog entry references a profile KubeAI
doesn't have, the resulting `Model` will fail to schedule; keep the two in sync
manually.

## User: picking a catalog model

1. The UI lists presets scoped to this provider: `GET /instance-presets?provider=provider-kubeai`.
2. The user picks one and gives the new Instance a name; the UI creates an `Instance`
   copying the preset's `spec` verbatim (same fields as `examples/instance-simple.yaml` /
   `examples/instance-gpu.yaml`).
3. Sync creates the `KubeAI Model` exactly as it would from a hand-written Instance;
   clients call `/openai/v1/...` as usual.

Power users can still bypass the catalog entirely and submit a raw Instance with any
`model.source` — presets are a convenience layer, not a restriction.

## Orthogonal: engine versions vs. catalog entries

`definition/versions.yaml` (`componentTypes.vllm.versions` + `versions` bundles)
defines which **engine** releases exist; `modelCatalog` entries reference a bundle by
name via `version:`. These are independent — adding a new curated model doesn't
require a new engine version, and vice versa. If a catalog entry needs an engine
release that isn't in `definition/versions.yaml` yet, add it there first (and run
`make generate`) before referencing it from `modelCatalog`.

## Open item

Whether the Everest UI already renders a preset picker against
`GET /instance-presets?provider=...` (vs. only the raw-source text field) is tracked
in the UI repo, not here. `provider-kubeai.openeverest.io/display-name` is set as an
annotation on each generated `InstancePreset` as a proposed convention for a
human-readable label — confirm against the UI repo whether it reads that or just uses
`metadata.name`, and adjust the template if not.
