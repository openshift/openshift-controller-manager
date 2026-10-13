# AI Agent Instructions for openshift-controller-manager

> Also read [ARCHITECTURE.md](ARCHITECTURE.md) for design decisions and
> [CONTRIBUTING.md](CONTRIBUTING.md) for workflow.

## What This Repo Is

The OpenShift Controller Manager (OCM) runs 19 controllers that reconcile OpenShift-specific API
resources: Builds, DeploymentConfigs, ImageStreams, TemplateInstances, Projects, plus cross-cutting
controllers for authorization, image pull secrets, and service unidling. It is deployed by the
[cluster-openshift-controller-manager-operator](https://github.com/openshift/cluster-openshift-controller-manager-operator).

## Repository Layout

```text
cmd/openshift-controller-manager/          # Main binary entrypoint
cmd/openshift-controller-manager-tests-ext/ # OTE test binary
pkg/cmd/controller/                        # Controller registry and init functions (start here)
pkg/cmd/openshift-controller-manager/      # Server bootstrap and config
pkg/apps/                                  # DeploymentConfig controllers (DEPRECATED)
pkg/build/                                 # Build controllers
pkg/image/                                 # ImageStream controllers and triggers
pkg/template/                              # TemplateInstance controllers
pkg/project/                               # Namespace finalizer controller
pkg/unidling/                              # Service unidling controller
pkg/authorization/                         # Default role binding controllers
pkg/internalregistry/                      # Internal registry pull secret controllers (6 sub-controllers)
vendor/                                    # Vendored dependencies
```

## Build and Test Commands

```bash
make build          # build binaries
make test-unit      # run unit tests
make verify         # gofmt, govet, version checks
```

## Critical Rules

1. **Never import `controller-runtime`** — this repo uses raw client-go. Mixing frameworks
   breaks informer sharing and creates subtle cache inconsistencies.
2. **Never modify `vendor/` in the same commit as code changes** — always commit vendor updates
   separately for reviewable diffs.
3. **DeploymentConfig is deprecated** — only critical and security fixes in `pkg/apps/`. Do not
   add features or refactor beyond what is required for the fix.
4. **Pull secret controllers are channel-coordinated** — the 6 sub-controllers in
   `pkg/internalregistry/` communicate via Go channels with blocking startup semantics.
   Changes to one controller may break the startup ordering of others.

## Key Patterns

- **Controller registration:** All controllers are registered in `pkg/cmd/controller/config.go`
  in the `ControllerInitializers` map. Each controller has an `InitFunc` in a corresponding file
  under `pkg/cmd/controller/`.
- **Per-controller service accounts:** Each controller runs with its own SA and scoped RBAC.
  SA names are constants in `pkg/cmd/controller/config.go`.
- **Capabilities integration:** Build and DC controllers can be disabled. Check
  `IsControllerEnabled()` and conditional informer startup in `StartInformers()`.
- **Error classification (apps):** `fatalError` (never retried), `actionableError` (retried with
  warning), regular errors (retried silently). Retry limits range from 5 to 15 by controller.
- **SSA in internalregistry:** Pull secret controllers use Server-Side Apply with distinct field
  manager strings to avoid conflicts.

## What NOT to Do

- Do not add new controllers without a corresponding entry in `ControllerInitializers` and a
  dedicated service account
- Do not start `AppsInformers` or `BuildInformers` unconditionally — they are gated on controller
  enablement to avoid unnecessary API watches
- Do not use status subresources for DeploymentConfig state — the annotation-driven state machine
  on ReplicationControllers is the API contract
- Do not delete or modify the `openshift.io/legacy-token` finalizer logic without understanding
  the rollback controller interaction

## Test Suites

- **Unit tests:** `make test-unit` — colocated `_test.go` files throughout `pkg/`
- **OTE:** `./openshift-controller-manager-tests-ext run-suite openshift/openshift-controller-manager/conformance/parallel`
- **E2E:** Run via `openshift/origin` against a live cluster (not in this repo)
