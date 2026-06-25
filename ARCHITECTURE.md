# Architecture: openshift-controller-manager

## Scope

The OpenShift Controller Manager (OCM) runs controllers that reconcile OpenShift-specific API
resources. It manages the lifecycle of Builds, DeploymentConfigs, ImageStreams, TemplateInstances,
and Projects, and provides cross-cutting controllers for authorization, image pull secrets, and
service unidling.

**Does not manage:** Routes (moved to [route-controller-manager](https://github.com/openshift/route-controller-manager) in 4.12),
resource quotas, or security context constraints (moved out during the 2019 split from openshift/origin).

## Namespace Map

| Namespace | Purpose |
|-----------|---------|
| `openshift-controller-manager` | Operand pods, serving-cert secrets, leader election Lease |
| `openshift-infra` | Service accounts used by controllers (e.g., `build-controller`, `deployer-controller`) |
| `openshift-config` | Cluster-wide CA bundles and proxy config consumed by the build controller |
| `openshift-image-registry` | Registry Service watched by the registry URL observation controller |
| `openshift-kube-apiserver` | Bound SA signing key secret watched by the key ID observation controller |

## Component Overview

The binary starts via `library-go`'s `NewControllerCommandConfig`, reads an
`OpenShiftControllerManagerConfig`, waits for a healthy API server, then starts each enabled
controller from a central registry of 19 controllers plus 1 rollback controller. Each controller
gets its own service account and rate-limited client. Leader election ensures single-active-instance
semantics.

## Controllers

| Controller | API Group | Watches | Reconciles |
|-----------|-----------|---------|------------|
| `openshift.io/deployer` | apps | RC, Pod | Creates deployer pods, tracks deployment lifecycle |
| `openshift.io/deploymentconfig` | apps | DC, RC | Triggers rollouts, manages RC replicas, cleans up old revisions |
| `openshift.io/build` | build | Build, Pod, IS, ConfigMaps, Secrets, cluster configs | Runs build lifecycle: resolves images, creates build pods, tracks completion |
| `openshift.io/build-config-change` | build | BuildConfig | Instantiates first build on ConfigChange trigger |
| `openshift.io/image-import` | image | ImageStream | One-shot imports for new/updated ImageStreams |
| `openshift.io/image-trigger` | image | IS, DC, BC, Deployment, DaemonSet, StatefulSet, CronJob | Propagates IS tag changes to dependent resources |
| `openshift.io/image-signature-import` | image | Image | Downloads container image signatures from registries |
| `openshift.io/templateinstance` | template | TemplateInstance | Processes templates, creates objects, polls readiness |
| `openshift.io/templateinstancefinalizer` | template | TemplateInstance | Deletes template-created objects on TI deletion |
| `openshift.io/origin-namespace` | project | Namespace | Removes `openshift.io/origin` finalizer during namespace deletion |
| `openshift.io/unidling` | — | Event (NeedPods) | Scales idled services back up on traffic |
| `openshift.io/serviceaccount` | — | SA | Provisions `builder` and `deployer` service accounts |
| `openshift.io/serviceaccount-pull-secrets` | — | SA, Secret, Service | Manages bound-token image pull secrets for the internal registry |
| `openshift.io/default-rolebindings` | — | Namespace, RoleBinding | Ensures `image-pullers`, `image-builders`, `deployers` bindings |
| `openshift.io/builder-serviceaccount` | — | SA | Provisions `builder` SA (capability-scoped) |
| `openshift.io/deployer-serviceaccount` | — | SA | Provisions `deployer` SA (capability-scoped) |
| `openshift.io/builder-rolebindings` | — | Namespace, RoleBinding | `system:image-builders` binding (capability-scoped) |
| `openshift.io/deployer-rolebindings` | — | Namespace, RoleBinding | `system:deployers` binding (capability-scoped) |
| `openshift.io/image-puller-rolebindings` | — | Namespace, RoleBinding | `system:image-pullers` binding (capability-scoped) |

The pull-secrets controller (`openshift.io/serviceaccount-pull-secrets`) is internally composed of
6 sub-controllers coordinated via Go channels: `ServiceAccountController`,
`ImagePullSecretController`, `RegistryURLObservationController`, `KeyIDObservationController`,
`LegacyImagePullSecretController`, and `LegacyTokenSecretController`.

**Rollback controllers:** The `RollbackControllers` map in `config.go` registers cleanup/rollback
logic that runs *instead of* a disabled controller. When `startControllers()` skips a disabled
controller, `startRollbackControllers()` checks this map and starts the corresponding rollback
init function. This pattern is extensible — any controller that needs cleanup behavior when
disabled can register a rollback entry. Currently only `serviceaccount-pull-secrets` has one
(the `legacyImagePullSecretRollbackController`).

## Capabilities Integration

Since OpenShift 4.14, Build and DeploymentConfig APIs can be disabled at install time via the
Capabilities API. Controllers were refactored into independently-disablable units:

- The monolithic service account controller was split into `builder-serviceaccount` and
  `deployer-serviceaccount`.
- The role bindings controller was split into `builder-rolebindings`, `deployer-rolebindings`,
  and `image-puller-rolebindings`.
- `AppsInformers` and `BuildInformers` only start when their controllers are enabled.
- The image trigger controller conditionally registers DC and BuildConfig sources.

## Manifest and Resource Management

OCM is deployed by the
[cluster-openshift-controller-manager-operator](https://github.com/openshift/cluster-openshift-controller-manager-operator),
not directly by the CVO. The operator manages the Deployment, ServiceAccount, ConfigMap
(controller config), and RBAC resources. OCM itself does not own CRDs — it consumes types defined
in [openshift/api](https://github.com/openshift/api).

## Dependencies

| Dependency | Role |
|-----------|------|
| `k8s.io/*` (v1.35) | Client-go, informers, workqueues, API machinery |
| `github.com/openshift/api` | OpenShift API type definitions |
| `github.com/openshift/client-go` | Typed OpenShift clients and informers |
| `github.com/openshift/library-go` | `controllercmd`, leader election, unidling client |
| `github.com/containers/image` | Image signature downloads (signature import controller) |

## Testing Strategy

- **Unit tests:** Colocated `_test.go` files in each package. Table-driven tests for build
  strategies, readiness checks, deployer pod creation, and pull secret controllers.
- **OTE (OpenShift Tests Extension):** Binary `openshift-controller-manager-tests-ext` shipped in
  the container image. Single-module architecture under `.openshift-tests-extension/`.
- **E2E:** Run via the `openshift/origin` test suite against a live cluster. No in-repo e2e tests.

## Design Decisions

1. **Split from openshift/origin (2018–2019):** OCM was extracted from the monolithic origin repo
   to enable independent release cycles. The git history carries the full origin lineage back to
   2014. Quota and SCC controllers were removed during this split.

2. **No controller-runtime:** OCM uses raw client-go informers and workqueues throughout. While
   this predates controller-runtime, it is also an active choice for performance and memory
   consumption. Raw informers allow precise control over which events trigger handlers — for
   example, the pull secret controllers filter by secret type and annotation at the handler level,
   avoiding unnecessary queue churn in clusters with large numbers of secrets and service accounts.

3. **Annotation-driven deployment state machine:** DeploymentConfig stores deployment status in RC
   annotations rather than a status subresource — a legacy pattern predating Kubernetes status
   conventions. This is load-bearing and cannot be changed without breaking the DC API contract.

4. **Per-controller service accounts:** Each controller runs with its own SA and scoped RBAC,
   following least-privilege. Client QPS is divided across controllers (original/10+1 per client),
   with high-rate controllers (pull secrets) getting a dedicated 100+ QPS path.

5. **Bucketed image import scheduler:** The `ScheduledImageStreamController` uses a custom bucketed
   scheduler instead of a standard workqueue, distributing import load evenly across time windows
   to avoid thundering-herd effects on external registries.

6. **Producer-consumer channel coordination for pull secrets:** The internal registry controllers
   use Go channels (not informers) to coordinate registry URL and signing key discovery. The pull
   secret controller blocks startup until both producers have emitted, preventing secret creation
   with incomplete registry information.

7. **DeploymentConfig is deprecated but permanent:** Deprecated since OCP 4.14 (OCPSTRAT-1465).
   Only critical and security fixes accepted. Depends on the upstream ReplicationController API.
   Removal version set to `v4.10000` — effectively never within the 4.x line.

8. **Route controllers extracted (4.12):** Moved to a standalone
   [route-controller-manager](https://github.com/openshift/route-controller-manager) process,
   partly driven by HyperShift's need for a separate deployment topology. Source code was fully
   removed from OCM on master (4.13). The Route API itself remains in openshift-apiserver for
   standard OCP, though MicroShift already serves routes as CRDs and shared validation is being
   consolidated in library-go.

9. **Capability-scoped controller splits (4.14–4.16):** Service account and role binding controllers
   were split from monolithic instances into per-capability units so Build and DeploymentConfig
   functionality can be individually disabled without affecting core image-puller bindings.

10. **Pull secret cleanup lives in the operator (tech debt):** When the internal image registry is
    disabled, the operator's `ImagePullSecretCleanupController` deletes managed pull secrets while
    simultaneously reconfiguring OCM to disable the creation controller. Because these are separate
    processes with different lifecycles, there is a race: OCM may still be running with old config
    and recreating secrets that the cleanup controller is deleting. A 10-minute grace period
    (OCPBUGS-34054) papers over this but is explicitly a stop-gap. The proper fix is to move
    cleanup into OCM as a rollback controller — the same pattern established for the
    `legacyImagePullSecretRollbackController` (PR #380, OCPBUGS-52193) — so that creation and
    deletion are mutually exclusive within the same process.
