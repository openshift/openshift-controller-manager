module github.com/openshift/openshift-controller-manager

go 1.14

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/blang/semver/v4 v4.0.0
	github.com/containers/image v3.0.2+incompatible
	github.com/containers/image/v5 v5.11.0
	github.com/coreos/go-systemd v0.0.0-20190321100706-95778dfbb74e
	github.com/davecgh/go-spew v1.1.1
	github.com/go-logr/logr v1.2.2 // indirect
	github.com/google/go-cmp v0.5.6
	github.com/google/gofuzz v1.2.0
	github.com/google/uuid v1.1.2
	github.com/hashicorp/golang-lru v0.5.1
	github.com/openshift/api v0.0.0-20220616165336-689617d54300
	github.com/openshift/build-machinery-go v0.0.0-20220429084610-baff9f8d23b3
	github.com/openshift/client-go v0.0.0-20220603133046-984ee5ebedcf
	github.com/openshift/library-go v0.0.0-20220615161831-8b2df431789c
	github.com/openshift/runtime-utils v0.0.0-20220513161558-c736ec4e99ce
	github.com/prometheus/client_golang v1.12.1
	github.com/prometheus/client_model v0.2.0
	github.com/spf13/cobra v1.4.0
	k8s.io/api v0.24.0
	k8s.io/apimachinery v0.24.0
	k8s.io/apiserver v0.24.0
	k8s.io/client-go v0.24.0
	k8s.io/component-base v0.24.0
	k8s.io/controller-manager v0.24.0
	k8s.io/klog/v2 v2.60.1
	k8s.io/kubectl v0.24.0
	k8s.io/kubernetes v1.24.0
	k8s.io/utils v0.0.0-20220210201930-3a6ce19ff2f9
)

replace (
	gopkg.in/square/go-jose.v2 => gopkg.in/square/go-jose.v2 v2.2.2
	// these are needed since k8s.io/kubernetes cites v0.0.0 for each of these k8s deps in its go.mod
	k8s.io/api => k8s.io/api v0.24.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.24.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.24.0
	k8s.io/apiserver => k8s.io/apiserver v0.24.0
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.24.0
	k8s.io/client-go => k8s.io/client-go v0.24.0
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.24.0
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.24.0
	k8s.io/code-generator => k8s.io/code-generator v0.24.0
	k8s.io/component-base => k8s.io/component-base v0.24.0
	k8s.io/component-helpers => k8s.io/component-helpers v0.24.0
	k8s.io/controller-manager => k8s.io/controller-manager v0.24.0
	k8s.io/cri-api => k8s.io/cri-api v0.24.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.24.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.24.0
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.24.0
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.24.0
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.24.0
	k8s.io/kubectl => k8s.io/kubectl v0.24.0
	k8s.io/kubelet => k8s.io/kubelet v0.24.0
	k8s.io/kubernetes => k8s.io/kubernetes v1.24.0
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.24.0
	k8s.io/metrics => k8s.io/metrics v0.24.0
	k8s.io/mount-utils => k8s.io/mount-utils v0.24.0
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.24.0
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.24.0
)
