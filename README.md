# OpenShift Controller Manager

The OpenShift Controller Manager (OCM) is comprised of multiple controllers, many of which
correspond to a top-level OpenShift API object, watching for changes and acting accordingly.
The controllers are generally organized by API group:

- `apps.openshift.io` - OpenShift-specific workloads, like `DeploymentConfig`.
- `build.openshift.io` - OpenShift `Builds` and `BuildConfigs`.
- `image.openshift.io` - `ImageStreams` and `Images`.
- `project.openshift.io` - Projects, OpenShift's wrapper for `Namespaces`.
- `template.openshift.io` - OpenShift `Templates` - a simple way to deploy applications.

There are additional controllers which add OpenShift-specific capabilities to the cluster:

- `authorization` - provides default service account role bindings for OpenShift projects.
- `serviceaccounts` - manages secrets that allow images to be pulled and pushed from the
  [OpenShift image registry](https://github.com/openshift/image-registry).
- `unidling` - manages unidling of applications when inbound network traffic is detected. See the
  [OpenShift docs](https://docs.openshift.com/container-platform/latest/applications/idling-applications.html#idle-unidling-applications_idling-applications)
  for more information.

## Metrics

Many of the controllers expose metrics which are visible in the default OpenShift monitoring system
(Prometheus). See [metrics](docs/metrics.md) for a detailed list of exposed metrics for each API
group.

## Rebase
Follow this checklist and copy into the PR:

- [ ] Select the desired [kubernetes release branch](https://github.com/kubernetes/kubernetes/branches), and use its `go.mod` and `CHANGELOG` as references for the rest of the work.
- [ ] Bump go version if needed.
- [ ] Bump `require`s and `replace`s for `k8s.io/`, `github.com/openshift/`, and relevant deps.
- [ ] Run `go mod vendor && go mod tidy`, commit `vendor` folder separately from all other changes.
- [ ] Bump image versions (Dockerfile, ci...) if needed.
- [ ] Run `make build verify test`.
- [ ] Make code changes as needed until the above pass.
- [ ] Any other minor update, like documentation.
## Tests

This repository is compatible with the "OpenShift Tests Extension (OTE)" framework.

### Building the test binary
```bash
make build
```

### Running test suites and tests
```bash
# Run a specific test suite or test
./openshift-controller-manager-tests-ext run-suite openshift/openshift-controller-manager/all
./openshift-controller-manager-tests-ext run-test "test-name"

# Run with JUnit output
./openshift-controller-manager-tests-ext run-suite openshift/openshift-controller-manager/all --junit-path=/tmp/junit-results/junit.xml
./openshift-controller-manager-tests-ext run-test "test-name" --junit-path=/tmp/junit-results/junit.xml
```

### Listing available tests and suites
```bash
# List all test suites
./openshift-controller-manager-tests-ext list-suites

# List tests in a specific suite
./openshift-controller-manager-tests-ext list-tests openshift/openshift-controller-manager/all
```

The test extension binary is included in the production image for CI/CD integration.
