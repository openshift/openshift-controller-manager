# OpenShift Controller Manager

The OpenShift Controller Manager (OCM) runs controllers that reconcile OpenShift-specific API
resources. It is a core control-plane component deployed by the
[cluster-openshift-controller-manager-operator](https://github.com/openshift/cluster-openshift-controller-manager-operator).

Controllers are organized by API group:

- `apps.openshift.io` — DeploymentConfig lifecycle and deployer pods
- `build.openshift.io` — Build and BuildConfig reconciliation
- `image.openshift.io` — ImageStream imports, triggers, and signature verification
- `project.openshift.io` — Project/namespace finalizer
- `template.openshift.io` — TemplateInstance processing and cleanup

Additional cross-cutting controllers handle default role bindings, image pull secret management
for the internal registry, and service unidling.

## Quick Start

### Prerequisites

- Go 1.25+
- An OpenShift cluster (OCM cannot run standalone)

### Building

```bash
make build          # builds the openshift-controller-manager binary
make verify         # runs gofmt, govet, and golang version checks
```

### Running Tests

```bash
make test-unit      # runs unit tests in ./pkg/... ./cmd/...
```

### OTE (OpenShift Tests Extension)

```bash
make build
./openshift-controller-manager-tests-ext list-suites
./openshift-controller-manager-tests-ext run-suite openshift/openshift-controller-manager/conformance/parallel
```

## Rebase Checklist

- [ ] Check the target [kubernetes release branch](https://github.com/kubernetes/kubernetes/branches) `go.mod` and `CHANGELOG`
- [ ] Bump Go version if needed
- [ ] Bump `k8s.io/`, `github.com/openshift/`, and relevant deps in `go.mod`
- [ ] Run `go mod tidy && go mod vendor`, commit vendor separately
- [ ] Bump image versions in Dockerfile and `.ci-operator.yaml` if needed
- [ ] Run `make build verify test-unit`
- [ ] Fix any compilation or test failures from upstream API changes

## Metrics

Controllers expose Prometheus metrics visible in the default OpenShift monitoring stack.
See [docs/metrics.md](docs/metrics.md) for the full list.

## Documentation

- [ARCHITECTURE.md](ARCHITECTURE.md) — Design decisions and component architecture
- [CONTRIBUTING.md](CONTRIBUTING.md) — How to submit changes
- [AGENTS.md](AGENTS.md) — AI agent instructions

## Related Repositories

- [cluster-openshift-controller-manager-operator](https://github.com/openshift/cluster-openshift-controller-manager-operator) — Operator that deploys OCM
- [openshift/api](https://github.com/openshift/api) — API type definitions
- [openshift/library-go](https://github.com/openshift/library-go) — Shared controller libraries
- [route-controller-manager](https://github.com/openshift/route-controller-manager) — Route controllers (extracted from OCM in 4.12)
- [openshift/origin](https://github.com/openshift/origin) — E2E test suite
