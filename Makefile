all: build
.PHONY: all

# Include the library makefile
include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
	golang.mk \
	targets/openshift/images.mk \
	targets/openshift/deps.mk \
)

export GO_BUILD_FLAGS="-gcflags=all=-N -l"

# Generate "images" targets
IMAGE_REGISTRY :=registry.ci.openshift.org/ocp/dev
$(call build-image,openshift-controller-manager,$(IMAGE_REGISTRY):openshift-controller-manager, ./Dockerfile.rhel,.)

clean:
	$(RM) ./openshift-controller-manager
.PHONY: clean

GO_TEST_PACKAGES :=./pkg/... ./cmd/...
