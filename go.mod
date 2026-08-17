module github.com/openshift/openshift-controller-manager

go 1.26.0

require (
	github.com/BurntSushi/toml v1.5.0
	github.com/blang/semver/v4 v4.0.0
	github.com/containers/image/v5 v5.30.1
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc
	github.com/google/go-cmp v0.7.0
	github.com/google/uuid v1.6.0
	github.com/openshift-eng/openshift-tests-extension v0.0.0-20260127124016-0fed2b824818
	github.com/openshift/api v0.0.0-20260813212709-d4bb0b443cb8
	github.com/openshift/build-machinery-go v0.0.0-20260629141115-154a2b810491
	github.com/openshift/client-go v0.0.0-20260810202730-ddca5e0b7146
	github.com/openshift/library-go v0.0.0-20260814203017-a0584a625d6d
	github.com/openshift/runtime-utils v0.0.0-20230921210328-7bdb5b9c177b
	github.com/prometheus/client_golang v1.23.2
	github.com/prometheus/client_model v0.6.2
	github.com/spf13/cobra v1.10.2
	golang.org/x/exp v0.0.0-20251219203646-944ab1f22d93
	gopkg.in/go-jose/go-jose.v2 v2.6.3
	k8s.io/api v0.36.3
	k8s.io/apimachinery v0.36.3
	k8s.io/apiserver v0.36.3
	k8s.io/client-go v0.36.3
	k8s.io/component-base v0.36.3
	k8s.io/controller-manager v0.36.3
	k8s.io/klog/v2 v2.140.0
	k8s.io/kubernetes v1.36.3
	k8s.io/utils v0.0.0-20260707023825-cf1189d6abe3
	sigs.k8s.io/randfill v1.0.0
)

require (
	cel.dev/expr v0.25.1 // indirect
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.0 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/containers/libtrust v0.0.0-20230121012942-c1716e8a8d01 // indirect
	github.com/containers/ocicrypt v1.1.9 // indirect
	github.com/containers/storage v1.53.0 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.7.0 // indirect
	github.com/cyberphone/json-canonicalization v0.0.0-20231217050601-ba74d44ecf5f // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker v25.0.3+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.8.1 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/felixge/fgprof v0.9.4 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/analysis v0.21.4 // indirect
	github.com/go-openapi/errors v0.21.1 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/loads v0.21.2 // indirect
	github.com/go-openapi/runtime v0.26.0 // indirect
	github.com/go-openapi/spec v0.20.9 // indirect
	github.com/go-openapi/strfmt v0.22.2 // indirect
	github.com/go-openapi/swag v0.25.4 // indirect
	github.com/go-openapi/swag/cmdutils v0.25.4 // indirect
	github.com/go-openapi/swag/conv v0.25.4 // indirect
	github.com/go-openapi/swag/fileutils v0.25.4 // indirect
	github.com/go-openapi/swag/jsonname v0.25.4 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.4 // indirect
	github.com/go-openapi/swag/loading v0.25.4 // indirect
	github.com/go-openapi/swag/mangling v0.25.4 // indirect
	github.com/go-openapi/swag/netutils v0.25.4 // indirect
	github.com/go-openapi/swag/stringutils v0.25.4 // indirect
	github.com/go-openapi/swag/typeutils v0.25.4 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.4 // indirect
	github.com/go-openapi/validate v0.22.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/cel-go v0.26.0 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-containerregistry v0.19.0 // indirect
	github.com/google/pprof v0.0.0-20260115054156-294ebfa9ad83 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus v1.1.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.3.3 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.7 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/letsencrypt/boulder v0.0.0-20230907030200-6d76a0f91e1e // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/user v0.4.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/opencontainers/runtime-spec v1.3.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pkg/profile v1.7.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/proglottis/gpgme v0.1.3 // indirect
	github.com/prometheus/common v0.67.5 // indirect
	github.com/prometheus/procfs v0.19.2 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.8.0 // indirect
	github.com/sigstore/fulcio v1.4.3 // indirect
	github.com/sigstore/rekor v1.2.2 // indirect
	github.com/sigstore/sigstore v1.8.2 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	github.com/titanous/rocacheck v0.0.0-20171023193734-afe73141d399 // indirect
	github.com/ulikunitz/xz v0.5.11 // indirect
	github.com/vbatts/tar-split v0.11.5 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.etcd.io/etcd/api/v3 v3.6.8 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.6.8 // indirect
	go.etcd.io/etcd/client/v3 v3.6.8 // indirect
	go.mongodb.org/mongo-driver v1.14.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.65.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.65.0 // indirect
	go.opentelemetry.io/otel v1.41.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.40.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.40.0 // indirect
	go.opentelemetry.io/otel/metric v1.41.0 // indirect
	go.opentelemetry.io/otel/sdk v1.40.0 // indirect
	go.opentelemetry.io/otel/trace v1.41.0 // indirect
	go.opentelemetry.io/proto/otlp v1.9.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/oauth2 v0.34.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/term v0.44.0 // indirect
	golang.org/x/text v0.39.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260128011058-8636f8732409 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260128011058-8636f8732409 // indirect
	google.golang.org/grpc v1.79.3 // indirect
	google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.36.2 // indirect
	k8s.io/component-helpers v0.33.2 // indirect
	k8s.io/kms v0.36.3 // indirect
	k8s.io/kube-openapi v0.0.0-20260519202549-bbf5c5577288 // indirect
	k8s.io/kubelet v0.29.1 // indirect
	k8s.io/streaming v0.36.3 // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.34.0 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.3 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

replace (
	// these are needed since k8s.io/kubernetes cites v0.0.0 for each of these k8s deps in its go.mod
	k8s.io/api => k8s.io/api v0.36.3
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.36.3
	k8s.io/apimachinery => k8s.io/apimachinery v0.36.3
	k8s.io/apiserver => k8s.io/apiserver v0.36.3
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.36.3
	// temporary bump to release-1.36:HEAD to add the missing https://github.com/kubernetes/client-go/commit/f4108cdac6d8de448edbd1ace56e38bde257ff56
	k8s.io/client-go => k8s.io/client-go v0.0.0-20260808041404-a92355f83186
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.36.3
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.36.3
	k8s.io/code-generator => k8s.io/code-generator v0.36.3
	k8s.io/component-base => k8s.io/component-base v0.36.3
	k8s.io/component-helpers => k8s.io/component-helpers v0.36.3
	k8s.io/controller-manager => k8s.io/controller-manager v0.36.3
	k8s.io/cri-api => k8s.io/cri-api v0.36.3
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.36.3
	k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.36.3
	k8s.io/endpointslice => k8s.io/endpointslice v0.36.3
	k8s.io/kms => k8s.io/kms v0.36.3
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.36.3
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.36.3
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.36.3
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.36.3
	k8s.io/kubectl => k8s.io/kubectl v0.36.3
	k8s.io/kubelet => k8s.io/kubelet v0.36.3
	k8s.io/kubernetes => k8s.io/kubernetes v1.36.3
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.36.3
	k8s.io/metrics => k8s.io/metrics v0.36.3
	k8s.io/mount-utils => k8s.io/mount-utils v0.36.3
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.36.3
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.36.3
)

replace (
	gopkg.in/go-jose/go-jose.v2 v2.6.0-unstable => github.com/go-jose/go-jose v2.6.3+incompatible
	gopkg.in/square/go-jose.v2 v2.6.0-unstable => github.com/go-jose/go-jose v2.6.3+incompatible
)
