# OpenShift Controller Manager

The OpenShift Controller Manager (OCM) is comprised of multiple controllers, many of which
correspond to a top-level OpenShift API object, watching for changes and acting accordingly.
The controllers are generally organized by API group:

- `apps.openshift.io` - OpenShift-specific workloads, like `DeploymentConfig`.
- `build.openshift.io` - OpenShift `Builds` and `BuildConfigs`.
- `image.openshift.io` - `ImageStreams` and `Images`.
- `project.openshift.io` - Projects, OpenShift's wrapper for `Namespaces`.
- `route.openshift.io` - OpenShift `Routes`, which provide similar capability to upstream `Ingress`
  objects.
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
