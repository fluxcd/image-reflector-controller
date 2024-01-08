GO_TEST_PATH ?= ./...
GO_TEST_ARGS ?=
PROVIDER_ARG ?=
TEST_TIMEOUT ?= 30m
FLUX_MANIFEST_URL ?= https://github.com/fluxcd/flux2/releases/latest/download/install.yaml
INSTALL_MANIFEST_PATH ?= build/flux/install.yaml

# image-reflector-controller test image.
TEST_IMG ?= fluxcd/image-reflector-controller:dev

$(INSTALL_MANIFEST_PATH):
	mkdir -p build/flux
	curl -Lo $(INSTALL_MANIFEST_PATH) $(FLUX_MANIFEST_URL)
	cp ../../config/crd/bases/* build/flux/

# Delete all the build files.
distclean:
	rm -rf build/

# Builds manifests and run the tests.
test: $(INSTALL_MANIFEST_PATH)
	# Check if the image exists locally.
	docker image inspect $(TEST_IMG) >/dev/null
	cp kustomization.yaml build/flux
	TEST_IMG=$(TEST_IMG) go test -timeout $(TEST_TIMEOUT) -v $(GO_TEST_PATH) $(GO_TEST_ARGS) $(PROVIDER_ARG)

test-aws:
	$(MAKE) test PROVIDER_ARG="-provider aws"

test-azure:
	$(MAKE) test PROVIDER_ARG="-provider azure"

test-gcp:
	$(MAKE) test PROVIDER_ARG="-provider gcp"

destroy:
	go test -timeout $(TEST_TIMEOUT) -v $(GO_TEST_PATH) $(GO_TEST_ARGS) $(PROVIDER_ARG) -destroy-only

destroy-aws:
	$(MAKE) destroy PROVIDER_ARG="-provider aws"

destroy-azure:
	$(MAKE) destroy PROVIDER_ARG="-provider azure"

destroy-gcp:
	$(MAKE) destroy PROVIDER_ARG="-provider gcp"
