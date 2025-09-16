all: build
.PHONY: all

# Include the library makefile
include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
	golang.mk \
	targets/openshift/images.mk \
	targets/openshift/deps.mk \
)

# Generate "images" targets
IMAGE_REGISTRY :=registry.ci.openshift.org/ocp/dev
$(call build-image,openshift-controller-manager,$(IMAGE_REGISTRY):openshift-controller-manager, ./Dockerfile.rhel,.)

clean:
	$(RM) ./openshift-controller-manager
.PHONY: clean

GO_TEST_PACKAGES :=./pkg/... ./cmd/...

# -------------------------------------------------------------------
# OpenShift Tests Extension (OpenShift Controller Manager)
# -------------------------------------------------------------------
TESTS_EXT_BINARY := openshift-controller-manager-tests-ext
TESTS_EXT_PACKAGE := ./cmd/openshift-controller-manager-tests-ext

TESTS_EXT_GIT_COMMIT := $(shell git rev-parse --short HEAD)
TESTS_EXT_BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
TESTS_EXT_GIT_TREE_STATE := $(shell if git diff-index --quiet HEAD --; then echo clean; else echo dirty; fi)

TESTS_EXT_LDFLAGS := \
	-X 'main.CommitFromGit=$(TESTS_EXT_GIT_COMMIT)' \
	-X 'main.BuildDate=$(TESTS_EXT_BUILD_DATE)' \
	-X 'main.GitTreeState=$(TESTS_EXT_GIT_TREE_STATE)'

.PHONY: tests-ext-build
tests-ext-build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) GO_COMPLIANCE_POLICY=exempt_all CGO_ENABLED=0 go build -o $(TESTS_EXT_BINARY) -ldflags "$(TESTS_EXT_LDFLAGS)" $(TESTS_EXT_PACKAGE)

.PHONY: tests-ext-update
tests-ext-update:
	./$(TESTS_EXT_BINARY) update

.PHONY: tests-ext-clean
tests-ext-clean:
	rm -f $(TESTS_EXT_BINARY) $(TESTS_EXT_BINARY).gz
