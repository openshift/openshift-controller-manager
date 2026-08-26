# Contributing to openshift-controller-manager

## Prerequisites

- Go 1.25+
- An OpenShift cluster (for e2e testing via openshift/origin)

## Development Workflow

1. Fork the repo and clone your fork
2. Create a feature branch from `master`
3. Make your changes, add or update tests
4. Run verification locally:
   ```bash
   make build verify test-unit
   ```
5. If you changed dependencies: `go mod tidy && go mod vendor` (commit vendor separately)
6. Push your branch and open a PR

## Pull Request Guidelines

- Keep PRs focused — one logical change per PR
- Reference JIRA tickets in the PR title: `OCPBUGS-XXXXX: description` or `CNTRLPLANE-XXXX: description`
- Include unit tests for new functionality
- PRs require `/lgtm` from a reviewer and `/approve` from an approver (see OWNERS files)
- All PRs require `/verified` before merge

## Building

| Command | What It Does |
|---------|-------------|
| `make build` | Builds the `openshift-controller-manager` and OTE test binaries |
| `make test-unit` | Runs unit tests in `./pkg/...` and `./cmd/...` |
| `make verify` | Runs `gofmt`, `govet`, and Go version checks |
| `make update-gofmt` | Auto-formats Go source files |
| `make image-openshift-controller-manager` | Builds the container image |
| `make vulncheck` | Runs govulncheck |

## Testing

Unit tests are colocated with source files. Run `make test-unit` to execute them.

E2E tests run via the [openshift/origin](https://github.com/openshift/origin) test suite against a
live cluster — there are no in-repo e2e tests.

The repo also ships an OTE binary (`openshift-controller-manager-tests-ext`) in the container
image. See the [README](README.md) for OTE usage.

## Code Conventions

- No `controller-runtime` — use raw `client-go` informers and workqueues
- Each controller gets its own service account and init function in `pkg/cmd/controller/`
- Retry limits range from 5 to 15 depending on the controller, with exponential backoff
- Use Server-Side Apply (SSA) with distinct field manager strings for new controllers
- Build and deployer pod specs live in `pkg/build/controller/strategy/` and
  `pkg/apps/deployer/` respectively

## Areas Requiring Extra Care

- **`vendor/`** — always commit separately from code changes; run `go mod tidy && go mod vendor`
- **Capabilities integration** — changes to controller registration must account for Build and
  DeploymentConfig being optional (disabled via Capabilities API)
- **Pull secret controllers** (`pkg/internalregistry/`) — coordinated via Go channels;
  changes to one sub-controller may affect startup ordering
- **DeploymentConfig** (`pkg/apps/`) — deprecated since OCP 4.14; only critical and security
  fixes accepted
- **Build pod security** — build pods run with specific security contexts; changes require
  careful review for privilege escalation

## CI Pipeline

CI runs via Prow and ci-operator. The build root image is configured in `.ci-operator.yaml`.
Full job definitions live in the [openshift/release](https://github.com/openshift/release)
repository.

## Review and Approval

The repo uses Prow's OWNERS-based review system. See the `OWNERS` and `OWNERS_ALIASES` files at
the repo root and in subdirectories for the current reviewer and approver lists. PRs touching
multiple areas may need approvers from each relevant OWNERS file.
