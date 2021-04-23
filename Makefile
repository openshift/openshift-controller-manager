all: build
.PHONY: all

# Include the library makefile
include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
	golang.mk \
	targets/openshift/images.mk \
	targets/openshift/deps.mk \
)

IMAGE_REGISTRY :=registry.svc.ci.openshift.org

# This will call a macro called "build-image" which will generate image specific targets based on the parameters:
# $0 - macro name
# $1 - target name
# $2 - image ref
# $3 - Dockerfile path
# $4 - context directory for image build# It will generate target "image-$(1)" for builing the image an binding it as a prerequisite to target "images".
$(call build-image,ocp-openshift-controller-manager,$(IMAGE_REGISTRY)/ocp/4.5:openshift-controller-manager, ./Dockerfile,.)

$(call verify-golang-versions,Dockerfile)

clean:
	$(RM) ./openshift-controller-manager
.PHONY: clean

GO_TEST_PACKAGES :=./pkg/... ./cmd/...
