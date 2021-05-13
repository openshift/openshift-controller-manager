FROM registry.ci.openshift.org/ocp/builder:golang-1.15 AS builder
WORKDIR /go/src/github.com/openshift/openshift-controller-manager
COPY . .
RUN make build --warn-undefined-variables

FROM registry.ci.openshift.org/openshift/origin-v4.0:base
COPY --from=builder /go/src/github.com/openshift/openshift-controller-manager/openshift-controller-manager /usr/bin/
LABEL io.k8s.display-name="OpenShift Controller Manager Command" \
      io.k8s.description="OpenShift is a platform for developing, building, and deploying containerized applications." \
      io.openshift.tags="openshift,openshift-controller-manager"
