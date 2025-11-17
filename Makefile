# ====================================================================================
# Setup Project

PROJECT_NAME := provider-http
PROJECT_REPO := github.com/crossplane-contrib/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64 linux_arm64

# -include will silently skip missing files, which allows us
# to load those files with a target in the Makefile. If only
# "include" was used, the make command would fail and refuse
# to run a target until the include commands succeeded.
-include build/makelib/common.mk

# ====================================================================================
# Setup Output

-include build/makelib/output.mk

# ====================================================================================
# Setup Go

# Set a sane default so that the nprocs calculation below is less noisy on the initial
# loading of this file
NPROCS ?= 1

# each of our test suites starts a kube-apiserver and running many test suites in
# parallel can lead to high CPU utilization. by default we reduce the parallelism
# to half the number of CPU cores.
GO_TEST_PARALLEL := $(shell echo $$(( $(NPROCS) / 2 )))

GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/provider
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.Version=$(VERSION)
GO_SUBDIRS += cmd internal apis
GO111MODULE = on
GOLANGCILINT_VERSION = 2.1.2
-include build/makelib/golang.mk

# ====================================================================================
# Setup Kubernetes tools
USE_HELM3 = true
CROSSPLANE_VERSION = 2.0.2
CROSSPLANE_CLI_VERSION = v2.0.2

-include build/makelib/k8s_tools.mk

# ====================================================================================
# Setup Images

IMAGES = provider-http
-include build/makelib/imagelight.mk

# ====================================================================================
# Targets

# run `make help` to see the targets and options

# We want submodules to be set up the first time `make` is run.
# We manage the build/ folder and its Makefiles as a submodule.
# The first time `make` is run, the includes of build/*.mk files will
# all fail, and this target will be run. The next time, the default as defined
# by the includes will be run instead.
fallthrough: submodules
	@echo Initial setup complete. Running make again . . .
	@make

# ====================================================================================
# Setup XPKG
XPKG_REG_ORGS ?= xpkg.upbound.io/crossplane-contrib
# NOTE(hasheddan): skip promoting on xpkg.upbound.io as channel tags are
# inferred.
XPKG_REG_ORGS_NO_PROMOTE ?= xpkg.upbound.io/crossplane-contrib
XPKGS = provider-http
-include build/makelib/xpkg.mk

# NOTE(hasheddan): we force image building to happen prior to xpkg build so that
# we ensure image is present in daemon.
xpkg.build.provider-http: do.build.images

# Generate a coverage report for cobertura applying exclusions on
# - generated file
cobertura:
	@cat $(GO_TEST_OUTPUT)/coverage.txt | \
		grep -v zz_generated.deepcopy | \
		$(GOCOVER_COBERTURA) > $(GO_TEST_OUTPUT)/cobertura-coverage.xml

# ====================================================================================
# End to End Testing
CROSSPLANE_NAMESPACE = crossplane-system
-include build/makelib/local.xpkg.mk
-include build/makelib/controlplane.mk

UPTEST_EXAMPLE_LIST := $(shell find ./examples/sample -path '*.yaml' | paste -s -d ',' - )

uptest: $(UPTEST) $(KUBECTL) $(CHAINSAW) $(CROSSPLANE_CLI)
	@$(INFO) running automated tests
	@KUBECTL=$(KUBECTL) CHAINSAW=$(CHAINSAW) CROSSPLANE_CLI=$(CROSSPLANE_CLI) CROSSPLANE_NAMESPACE=$(CROSSPLANE_NAMESPACE) $(UPTEST) e2e "$(UPTEST_EXAMPLE_LIST)" --setup-script=cluster/test/setup.sh || $(FAIL)
	@$(OK) running automated tests

local-dev: controlplane.up
local-deploy: build controlplane.up local.xpkg.deploy.provider.$(PROJECT_NAME)
	@$(INFO) running locally built provider
	@if kubectl config current-context | grep -q "kind-"; then \
		CLUSTER_NAME=$$(kubectl config current-context | sed 's/kind-//'); \
		echo "Loading test server image into kind cluster: $$CLUSTER_NAME"; \
		$(KIND) load docker-image $(TEST_SERVER_IMAGE) --name $$CLUSTER_NAME || echo "Warning: Failed to load test-server image"; \
	fi
	@$(KUBECTL) wait provider.pkg $(PROJECT_NAME) --for condition=Healthy --timeout 5m
	@$(KUBECTL) -n $(CROSSPLANE_NAMESPACE) wait --for=condition=Available deployment --all --timeout=5m
	@$(OK) running locally built provider

# Prepare for E2E testing - always rebuild test server
e2e.prepare: test-server.build
	@$(INFO) preparing for e2e tests
	@echo "✅ Test server image $(TEST_SERVER_IMAGE) is ready"
	@$(OK) preparing for e2e tests

# Main E2E target - builds test server, deploys provider, runs tests
e2e: e2e.prepare local-deploy uptest

# ====================================================================================
# Local Test Server Development

# Test server configuration
# CI can override this via environment variable
# For local development, this is built by test-server.build target
TEST_SERVER_IMAGE ?= provider-http-test-server:latest
TEST_SERVER_CONTAINER = provider-http-test-server
TEST_SERVER_PORT = 5001

.PHONY: test-server.build test-server.rebuild test-server.start test-server.stop test-server.restart test-server.logs test-server.status test-server.clean

# BUILD_ARGS can be set to add docker build flags (e.g., BUILD_ARGS="--load" for buildx)
BUILD_ARGS ?=

test-server.build:
	@$(INFO) building test server image
	@cd cluster/test && docker build $(BUILD_ARGS) -t $(TEST_SERVER_IMAGE) .
	@$(OK) building test server image

# Force rebuild test server image (bypass cache)
test-server.rebuild:
	@$(INFO) rebuilding test server image (no cache)
	@cd cluster/test && docker build $(BUILD_ARGS) --no-cache -t $(TEST_SERVER_IMAGE) .
	@$(OK) rebuilding test server image

test-server.start: test-server.build
	@$(INFO) starting test server container
	@docker run -d --name $(TEST_SERVER_CONTAINER) -p $(TEST_SERVER_PORT):5000 $(TEST_SERVER_IMAGE) || \
		(echo "Container may already exist. Use 'make test-server.restart' to restart it." && exit 1)
	@echo "Test server starting at http://localhost:$(TEST_SERVER_PORT)"
	@echo "Waiting for server to be ready..."
	@sleep 3
	@curl -f -H "Authorization: Bearer my-secret-value" -X POST http://localhost:$(TEST_SERVER_PORT)/v1/login > /dev/null && \
		echo "✅ Test server is ready!" || echo "❌ Test server may not be ready yet"
	@$(OK) starting test server container

test-server.stop:
	@$(INFO) stopping test server container
	@docker stop $(TEST_SERVER_CONTAINER) 2>/dev/null || echo "Container not running"
	@docker rm $(TEST_SERVER_CONTAINER) 2>/dev/null || echo "Container not found"
	@$(OK) stopping test server container

test-server.restart: test-server.stop test-server.start

test-server.logs:
	@echo "=== Test Server Logs ==="
	@docker logs $(TEST_SERVER_CONTAINER) 2>/dev/null || echo "Container not found or not running"

test-server.status:
	@echo "=== Test Server Status ==="
	@docker ps -f name=$(TEST_SERVER_CONTAINER) --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" || echo "Container not found"
	@echo ""
	@echo "Testing server health..."
	@curl -f -H "Authorization: Bearer my-secret-value" -X POST http://localhost:$(TEST_SERVER_PORT)/v1/login 2>/dev/null && \
		echo "✅ Server is healthy" || echo "❌ Server is not responding"

test-server.clean: test-server.stop
	@$(INFO) cleaning test server image
	@docker rmi $(TEST_SERVER_IMAGE) 2>/dev/null || echo "Image not found"
	@$(OK) cleaning test server image

test-server.help:
	@echo "Test Server Development Targets:"
	@echo "  test-server.build    - Build the test server Docker image"
	@echo "  test-server.start    - Start the test server container"
	@echo "  test-server.stop     - Stop and remove the test server container"
	@echo "  test-server.restart  - Restart the test server (stop + start)"
	@echo "  test-server.logs     - Show test server logs"
	@echo "  test-server.status   - Show container status and health"
	@echo "  test-server.clean    - Stop container and remove image"
	@echo "  test-server.help     - Show this help"
	@echo ""
	@echo "E2E Testing Targets:"
	@echo "  e2e                  - Run E2E tests (original target)"
	@echo "  e2e.prepare          - Check/prepare test server image for E2E"
	@echo "  e2e.local            - Prepare environment and run E2E tests"
	@echo ""
	@echo "Server runs on: http://localhost:$(TEST_SERVER_PORT)"
	@echo "Authorization: Bearer my-secret-value"
	@echo "Current TEST_SERVER_IMAGE: $(TEST_SERVER_IMAGE)"

# Update the submodules, such as the common build scripts.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

# NOTE(hasheddan): we must ensure up is installed in tool cache prior to build
# as including the k8s_tools machinery prior to the xpkg machinery sets
# CROSSPLANE_CLI to point to tool cache.
build.init: $(CROSSPLANE_CLI)

# This is for running out-of-cluster locally, and is for convenience. Running
# this make target will print out the command which was used. For more control,
# try running the binary directly with different arguments.
run: $(KUBECTL) generate
	@$(INFO) Running Crossplane locally out-of-cluster . . .
	@$(KUBECTL) apply -f package/crds/ -R
	go run cmd/provider/main.go -d

manifests:
	@$(INFO) Deprecated. Run make generate instead.

# NOTE(hasheddan): the build submodule currently overrides XDG_CACHE_HOME in
# order to force the Helm 3 to use the .work/helm directory. This causes Go on
# Linux machines to use that directory as the build cache as well. We should
# adjust this behavior in the build submodule because it is also causing Linux
# users to duplicate their build cache, but for now we just make it easier to
# identify its location in CI so that we cache between builds.
go.cachedir:
	@go env GOCACHE

go.mod.cachedir:
	@go env GOMODCACHE

.PHONY: cobertura submodules fallthrough test-integration run manifests go.mod.cachedir go.cachedir

vendor: modules.download
vendor.check: modules.check
