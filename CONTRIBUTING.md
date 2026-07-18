# Contributing to provider-kubeai

Thanks for your interest in contributing! This repo is an OpenEverest **provider**:
a Kubernetes controller that translates OpenEverest `Instance` CRs into
[KubeAI](https://www.kubeai.org/) `Model` CRs for LLM serving (vLLM on GPU, Ollama
on CPU). See [README.md](README.md) for what it does and how to run it locally.

## Before you start

- For small fixes (typos, docs, small bugs), just open a PR.
- For larger changes (new topology, new component customSpec fields, changes to
  what this provider owns on the KubeAI `Model`), open an issue first using the
  [feature request template](.github/ISSUE_TEMPLATE/feature_request.yml) so we can
  align on approach before you invest the work.
- For changes that affect OpenEverest's architecture more broadly (not just this
  provider), propose them in [openeverest/specs](https://github.com/openeverest/specs)
  first — see [Related Repositories](#related-repositories) below.

## Local setup

Prerequisites: Go (version pinned in `go.mod`), Docker, Helm, kubectl.

Bring up a local cluster (KIND or k3d — see [docs/local-kind-runbook.md](docs/local-kind-runbook.md)
or [docs/local-k3d-runbook.md](docs/local-k3d-runbook.md)), install KubeAI into it, then:

```sh
make generate   # regenerate RBAC / Helm chart / provider spec from definition/
make run        # run the provider locally against your cluster
```

Try it end-to-end with the example Instances in `examples/` (see README's
"Create an Instance" section).

## Making changes

- **Provider logic** (`Validate`/`Sync`/`Status`/`Cleanup`) lives in `internal/provider/`.
- **Public schema** (topology config, global config, component customSpec) lives in
  `definition/` and is tagged with `+kubebuilder:validation` / `+k8s:openapi-gen=true`
  markers — these generate the Helm chart's CRD schema, so typos in markers fail silently
  rather than erroring.
- If you touch `definition/` or the RBAC markers in `internal/provider/rbac.go`, run
  `make generate` and commit the resulting changes (chart, `config/rbac/role.yaml`).
- Keep `Sync` idempotent: only set fields this provider owns, and never set or clear
  `Model.Spec.Replicas` — KubeAI's autoscaler owns that field.

## Before opening a PR

Run the same checks CI runs (`.github/workflows/ci.yaml`):

```sh
make lint      # golangci-lint
go build ./... # or: make build
make test      # unit tests
make verify    # fails if generated files (chart, RBAC) are out of date
helm lint charts/provider-kubeai
```

## Commit / PR style

- Keep PRs focused; unrelated cleanups belong in a separate PR.
- Write commit messages that explain *why*, not just *what*.
- Link the issue you're addressing, if any.

## Reporting bugs / requesting features

Use the [issue templates](.github/ISSUE_TEMPLATE/) — bug reports ask you to pick an
area (Sync/Status/Validate, Helm, UI, autoscaling, inference API, local dev, docs) which
helps route the issue to the right place.

## Related Repositories

- **[openeverest/openeverest](https://github.com/openeverest/openeverest)** — the
  OpenEverest core: the `Instance`/`Provider` CRDs and controller runtime
  (`provider-runtime`) this provider is built on.
- **[openeverest/provider-sdk](https://github.com/openeverest/provider-sdk)** — the SDK
  used by `gen.go`'s `go:generate` directive to turn `definition/` into the Provider CR
  spec and Helm chart.
- **[openeverest/specs](https://github.com/openeverest/specs)** — OpenEverest Specs, the
  central hub for proposing, defining, and archiving significant features and
  architectural decisions across the project. Use it for major proposals, architecture
  and design docs, roadmapping, and structured discussion on project direction — not for
  changes scoped to this provider alone.

## Maintainers

See [MAINTAINERS.md](MAINTAINERS.md) for who reviews and merges changes here.
