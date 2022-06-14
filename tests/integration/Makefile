GO_TEST_PATH ?= ./...
GO_TEST_ARGS ?=
PROVIDER_ARG ?=
TEST_TIMEOUT ?= 30m
FLUX_MANIFEST_URL ?= https://github.com/fluxcd/flux2/releases/latest/download/install.yaml
INSTALL_MANIFEST_PATH ?= build/flux/install.yaml

IMG ?= fluxcd/image-reflector-controller

$(INSTALL_MANIFEST_PATH):
	mkdir -p build/flux
	curl -Lo $(INSTALL_MANIFEST_PATH) $(FLUX_MANIFEST_URL)

# Build the manifests required in the test.
build-manifests: $(INSTALL_MANIFEST_PATH)
	cp kustomization.yaml build/flux
	cd build/flux && kustomize edit set image fluxcd/image-reflector-controller=${IMG}
	kustomize build build/flux > build/flux.yaml

# Delete all the build files.
distclean:
	rm -r build/

# Builds manifests and run the tests.
test: build-manifests
	go test -timeout $(TEST_TIMEOUT) -v $(GO_TEST_PATH) $(GO_TEST_ARGS) $(PROVIDER_ARG)

PROVIDER_ARG=-provider aws
test-aws: test
