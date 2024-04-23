# Set up GOBIN so that our binaries are installed to ./bin instead of $GOPATH/bin.
PROJECT_ROOT = $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
export GOBIN = $(PROJECT_ROOT)/bin

GOLANGCI_LINT_VERSION := $(shell $(GOBIN)/golangci-lint version --format short 2>/dev/null)
REQUIRED_GOLANGCI_LINT_VERSION := $(shell cat .golangci.version)

# Directories containing independent Go modules.
MODULE_DIRS = . ./tools

.PHONY: all
all: build lint test integration-test

.PHONY: clean
clean:
	@rm -rf $(GOBIN)

.PHONY: build
build:
	go install go.uber.org/nilaway/cmd/nilaway

.PHONY: test
test:
	@$(foreach mod,$(MODULE_DIRS),(cd $(mod) && go test -race ./...) &&) true

.PHONY: cover
cover:
	@$(foreach mod,$(MODULE_DIRS), ( \
		cd $(mod) && \
		go test -race -coverprofile=cover.out -coverpkg=./... ./... \
		&& go tool cover -html=cover.out -o cover.html) &&) true

.PHONY: golden-test
golden-test:
	@cd tools && go install go.uber.org/nilaway/tools/cmd/golden-test
	@$(GOBIN)/golden-test $(ARGS)

.PHONY: integration-test
integration-test:
	@cd tools && go install go.uber.org/nilaway/tools/cmd/integration-test
	@$(GOBIN)/integration-test

.PHONY: lint
lint: golangci-lint nilaway-lint tidy-lint

# Install golangci-lint with the required version in GOBIN if it is not already installed.
.PHONY: install-golangci-lint
install-golangci-lint:
    ifneq ($(GOLANGCI_LINT_VERSION),$(REQUIRED_GOLANGCI_LINT_VERSION))
		@echo "[lint] installing golangci-lint v$(REQUIRED_GOLANGCI_LINT_VERSION) since current version is \"$(GOLANGCI_LINT_VERSION)\""
		@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOBIN) v$(REQUIRED_GOLANGCI_LINT_VERSION)
    endif

.PHONY: golangci-lint
golangci-lint: install-golangci-lint
	@echo "[lint] $(shell $(GOBIN)/golangci-lint version)"
	@$(foreach mod,$(MODULE_DIRS), \
		(cd $(mod) && \
		echo "[lint] golangci-lint: $(mod)" && \
		$(GOBIN)/golangci-lint run --path-prefix $(mod)) &&) true

.PHONY: tidy-lint
tidy-lint:
	@$(foreach mod,$(MODULE_DIRS), \
		(cd $(mod) && \
		echo "[lint] mod tidy: $(mod)" && \
		go mod tidy && \
		git diff --exit-code -- go.mod go.sum) &&) true

.PHONY: nilaway-lint
nilaway-lint: build
	@$(foreach mod,$(MODULE_DIRS), \
		(cd $(mod) && \
		echo "[lint] nilaway linting itself: $(mod)" && \
		$(GOBIN)/nilaway -include-pkgs="go.uber.org/nilaway" ./...) &&) true
