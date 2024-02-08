# Set up GOBIN so that our binaries are installed to ./bin instead of $GOPATH/bin.
PROJECT_ROOT = $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
export GOBIN = $(PROJECT_ROOT)/bin

GOLANGCI_LINT_VERSION := $(shell golangci-lint --version 2>/dev/null)

# Directories containing independent Go modules.
MODULE_DIRS = . ./tools

.PHONY: all
all: build lint test

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

.PHONY: golangci-lint
golangci-lint:
ifdef GOLANGCI_LINT_VERSION
	@echo "[lint] $(GOLANGCI_LINT_VERSION)"
else
	$(error "golangci-lint not found, please install it from https://golangci-lint.run/usage/install/#local-installation")
endif
	@$(foreach mod,$(MODULE_DIRS), \
		(cd $(mod) && \
		echo "[lint] golangci-lint: $(mod)" && \
		golangci-lint run --path-prefix $(mod)) &&) true

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
