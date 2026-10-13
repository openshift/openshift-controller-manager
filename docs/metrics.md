# Openshift Controller Manager Metrics

OCM exposes the following metrics to Prometheus for monitoring and analysis:

## Apps

| Name | Type | Labels | Description |
| ---- | ---- | ------ | ----------- |
| `openshift_apps_deploymentconfigs_active_rollouts_duration_seconds` | Counter | `namespace`, `name`, `phase`, `latest_version` | Tracks the active rollout duration in seconds |
| `openshift_apps_deploymentconfigs_complete_rollouts_total` | Gauge | `phase` | Counts total complete rollouts |
| `openshift_apps_deploymentconfigs_last_failed_rollout_time` | Gauge | `namespace`, `name`, `latest_version` | Tracks the active rollout duration in seconds |

## Builds

| Name | Type | Labels | Description |
| ---- | ---- | ------ | ----------- |
| `openshift_build_total` | Gauge | `phase`, `reason`, `strategy` | Counts builds by phase, reason, and strategy |
| `openshift_build_active_time_seconds` | Gauge | `namespace`, `name`, `phase`, `reason`, `strategy` | Shows the last transition time in unix epoch for running builds by namespace, name, phase, reason, and strategy |

## Image

| Name | Type | Labels | Description |
| ---- | ---- | ------ | ----------- |
| `openshift_imagestreamcontroller_error_count` | Counter | `scheduled`, `registry`, `reason` | Counts number of failed image stream imports - both scheduled and not scheduled - per image registry and failure reason |
| `openshift_imagestreamcontroller_success_count` | Counter | `scheduled`, `registry` | Counts successful image stream imports - both scheduled and not scheduled - per image registry |

## Templates

| Name | Type | Labels | Description |
| ---- | ---- | ------ | ----------- |
| `openshift_template_instance_completed_total` | Counter | `condition` | Counts completed TemplateInstance objects by condition |
| `openshift_template_instance_active_age_seconds` | Histogram | No labels | Shows the instantaneous age distribution of active TemplateInstance objects |

## Unidling
| Name | Type | Labels | Description |
| ---- | ---- | ------ | ----------- |
| `openshift_unidle_events_total` | Counter | No labels | Total count of unidling events observed by the unidling controller |
