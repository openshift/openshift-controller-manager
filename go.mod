module github.com/openshift/openshift-controller-manager

go 1.12

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/BurntSushi/toml v0.3.1
	github.com/MakeNowJust/heredoc v0.0.0-20170808103936-bb23615498cd // indirect
	github.com/Microsoft/go-winio v0.4.11 // indirect
	github.com/NYTimes/gziphandler v0.0.0-20170623195520-56545f4a5d46 // indirect
	github.com/blang/semver v3.5.0+incompatible // indirect
	github.com/certifi/gocertifi v0.0.0-20180905225744-ee1a9a0726d2 // indirect
	github.com/containerd/continuity v0.0.0-20190827140505-75bee3e2ccb6 // indirect
	github.com/containers/image v3.0.2+incompatible
	github.com/containers/storage v0.0.0-20170801145921-47536c89fcc5 // indirect
	github.com/coreos/bbolt v1.3.1-coreos.6 // indirect
	github.com/coreos/go-systemd v0.0.0-20180511133405-39ca1b05acc7
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/docker/distribution v0.0.0-20180920194744-16128bbac47f // indirect
	github.com/docker/docker v0.0.0-20180612054059-a9fbbdc8dd87 // indirect
	github.com/docker/docker-credential-helpers v0.0.0-20190720063934-f78081d1f7fe // indirect
	github.com/docker/go-connections v0.3.0 // indirect
	github.com/docker/go-metrics v0.0.0-20181218153428-b84716841b82 // indirect
	github.com/docker/go-units v0.0.0-20170127094116-9e638d38cf69 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/docker/spdystream v0.0.0-20160310174837-449fdfce4d96 // indirect
	github.com/elazarl/goproxy v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/emicklei/go-restful v0.0.0-20170410110728-ff4f55a20633 // indirect
	github.com/evanphx/json-patch v4.2.0+incompatible // indirect
	github.com/exponent-io/jsonpath v0.0.0-20151013193312-d6023ce2651d // indirect
	github.com/getsentry/raven-go v0.0.0-20171206001108-32a13797442c // indirect
	github.com/ghodss/yaml v0.0.0-20150909031657-73d445a93680 // indirect
	github.com/go-openapi/jsonpointer v0.19.0 // indirect
	github.com/go-openapi/jsonreference v0.19.0 // indirect
	github.com/go-openapi/spec v0.17.2 // indirect
	github.com/go-openapi/swag v0.17.2 // indirect
	github.com/golang/groupcache v0.0.0-20160516000752-02826c3e7903 // indirect
	github.com/google/btree v0.0.0-20190326150332-20236160a414 // indirect
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/google/gofuzz v0.0.0-20170612174753-24818f796faf
	github.com/google/uuid v0.0.0-20190416172445-c2e93f3ae59f
	github.com/googleapis/gnostic v0.0.0-20170729233727-0c5108395e2d // indirect
	github.com/gorilla/mux v0.0.0-20190108142930-08e7f807d38d // indirect
	github.com/gorilla/websocket v1.4.1 // indirect
	github.com/gotestyourself/gotestyourself v2.2.0+incompatible // indirect
	github.com/gregjones/httpcache v0.0.0-20170728041850-787624de3eb7 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.1.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v0.0.0-20170330212424-2500245aa611 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.3.0 // indirect
	github.com/hashicorp/golang-lru v0.5.0
	github.com/imdario/mergo v0.3.5 // indirect
	github.com/jonboulle/clockwork v0.1.0 // indirect
	github.com/json-iterator/go v0.0.0-20180701071628-ab8a2e0c74be // indirect
	github.com/mitchellh/go-wordwrap v0.0.0-20150314170334-ad45545899c7 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/mtrmac/gpgme v0.0.0-20170102180018-b2432428689c // indirect
	github.com/munnerz/goautoneg v0.0.0-20120707110453-a547fc61f48d // indirect
	github.com/onsi/ginkgo v1.10.2 // indirect
	github.com/onsi/gomega v1.7.0 // indirect
	github.com/opencontainers/go-digest v0.0.0-20170106003457-a6d0ee40d420 // indirect
	github.com/opencontainers/image-spec v0.0.0-20170604055404-372ad780f634 // indirect
	github.com/opencontainers/runc v0.0.0-20181113202123-f000fe11ece1 // indirect
	github.com/openshift/api v0.0.0-20190904155310-a25bb2adc83e
	github.com/openshift/client-go v0.0.0-20190813201236-5a5508328169
	github.com/openshift/library-go v0.0.0-20190904120025-7d4acc018c61
	github.com/openshift/machine-config-operator v0.0.0-20190904184504-49d703c2c17a
	github.com/pborman/uuid v0.0.0-20150603214016-ca53cad383ca // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pkg/profile v1.3.0 // indirect
	github.com/prometheus/client_golang v0.9.2
	github.com/prometheus/client_model v0.0.0-20180712105110-5c3871d89910
	github.com/prometheus/common v0.2.0 // indirect
	github.com/soheilhy/cmux v0.1.4 // indirect
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.3
	github.com/tmc/grpc-websocket-proxy v0.0.0-20190109142713-0ad062ec5ee5 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2 // indirect
	golang.org/x/time v0.0.0-20161028155119-f51c12702a4d // indirect
	gopkg.in/inf.v0 v0.9.0 // indirect
	gopkg.in/square/go-jose.v2 v2.0.0-20180411045311-89060dee6a84 // indirect
	gotest.tools v2.2.0+incompatible // indirect
	k8s.io/api v0.0.0-20190313235455-40a48860b5ab
	k8s.io/apiextensions-apiserver v0.0.0-20190315093550-53c4693659ed // indirect
	k8s.io/apimachinery v0.0.0-20190313205120-d7deff9243b1
	k8s.io/apiserver v0.0.0-20190313205120-8b27c41bdbb1
	k8s.io/cli-runtime v0.0.0-20190314001948-2899ed30580f // indirect
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/cloud-provider v0.0.0-20190314002645-c892ea32361a // indirect
	k8s.io/component-base v0.0.0-20190314000054-4a91899592f4
	k8s.io/klog v0.0.0-20181108234604-8139d8cb77af
	k8s.io/kube-openapi v0.0.0-20190228160746-b3a7cee44a30 // indirect
	k8s.io/kubernetes v1.14.0
	k8s.io/utils v0.0.0-20190221042446-c2654d5206da
	sigs.k8s.io/kustomize v2.0.3+incompatible // indirect
	sigs.k8s.io/structured-merge-diff v0.0.0-20190302045857-e85c7b244fd2 // indirect
	sigs.k8s.io/yaml v1.1.0 // indirect
)
