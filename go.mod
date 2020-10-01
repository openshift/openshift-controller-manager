module github.com/openshift/openshift-controller-manager

go 1.14

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/blang/semver v3.5.0+incompatible
	github.com/containers/image v3.0.2+incompatible
	github.com/containers/storage v0.0.0-20170801145921-47536c89fcc5 // indirect
	github.com/coreos/go-systemd v0.0.0-20190321100706-95778dfbb74e
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/docker-credential-helpers v0.0.0-20190720063934-f78081d1f7fe // indirect
	github.com/google/go-cmp v0.4.0
	github.com/google/gofuzz v1.1.0
	github.com/google/uuid v1.1.1
	github.com/gorilla/websocket v1.4.1 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.1.0 // indirect
	github.com/hashicorp/golang-lru v0.5.1
	github.com/mtrmac/gpgme v0.1.2 // indirect
	github.com/openshift/api v0.0.0-20200723134351-89de68875e7c
	github.com/openshift/build-machinery-go v0.0.0-20200713135615-1f43d26dccc7
	github.com/openshift/client-go v0.0.0-20200722173614-5a1b0aaeff15
	github.com/openshift/library-go v0.0.0-20200731134909-dbf343342338
	github.com/openshift/runtime-utils v0.0.0-20200415173359-c45d4ff3f912
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/client_model v0.2.0
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	go.uber.org/zap v1.11.0 // indirect
	k8s.io/api v0.19.0-rc.3
	k8s.io/apimachinery v0.19.0-rc.3
	k8s.io/apiserver v0.19.0-rc.3
	k8s.io/client-go v0.19.0-rc.3
	k8s.io/component-base v0.19.0-rc.3
	k8s.io/klog/v2 v2.2.0
	k8s.io/kubectl v0.19.0-rc.3
	k8s.io/kubernetes v1.19.0-rc.3
	k8s.io/utils v0.0.0-20200720150651-0bdb4ca86cbc
)

replace (
	// these are needed since k8s.io/kubernetes cites v0.0.0 for each of these k8s deps in its go.mod
	k8s.io/api => k8s.io/api v0.19.0-rc.3
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.0-rc.3
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.0-rc.3
	k8s.io/apiserver => k8s.io/apiserver v0.19.0-rc.3
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.19.0-rc.3
	k8s.io/client-go => k8s.io/client-go v0.19.0-rc.3
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.19.0-rc.3
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.19.0-rc.3
	k8s.io/code-generator => k8s.io/code-generator v0.19.0-rc.3
	k8s.io/component-base => k8s.io/component-base v0.19.0-rc.3
	k8s.io/cri-api => k8s.io/cri-api v0.19.0-rc.3
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.19.0-rc.3
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.19.0-rc.3
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.19.0-rc.3
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.19.0-rc.3
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.19.0-rc.3
	k8s.io/kubectl => k8s.io/kubectl v0.19.0-rc.3
	k8s.io/kubelet => k8s.io/kubelet v0.19.0-rc.3
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.19.0-rc.3
	k8s.io/metrics => k8s.io/metrics v0.19.0-rc.3
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.19.0-rc.3
)
