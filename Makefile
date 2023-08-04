# Set up GOBIN so that our binaries are installed to ./bin instead of $GOPATH/bin.
PROJECT_ROOT = $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
export GOBIN = $(PROJECT_ROOT)/bin

GOLANGCI_LINT_VERSION := $(shell golangci-lint --version 2>/dev/null)

.PHONY: all
all: build lint test

.PHONY: build
build:
	go install go.uber.org/nilaway/cmd/nilaway

.PHONY: test
test:
	go test -v -race ./...

.PHONY: cover
cover:
	go test -v -race -coverprofile=cover.out -coverpkg=./... -v ./...
	go tool cover -html=cover.out -o cover.html

.PHONY: lint
lint: golangci-lint nilaway-lint tidy-lint

.PHONY: golangci-lint
golangci-lint:
ifdef GOLANGCI_LINT_VERSION
	@echo "[lint] $(GOLANGCI_LINT_VERSION)"
else
	$(error "golangci-lint not found, please install it from https://golangci-lint.run/usage/install/#local-installation")
endif
	@echo "[lint] golangci-lint run"
	@golangci-lint run

.PHONY: tidy-lint
tidy-lint:
	@echo "[lint] go mod tidy"
	@go mod tidy && \
		git diff --exit-code -- go.mod go.sum || \
		(echo "'go mod tidy' changed files" && false)

.PHONY: nilaway-lint
nilaway-lint: build
	@echo "[lint] nilaway linting itself"
	@$(GOBIN)/nilaway -include-pkgs="go.uber.org/nilaway" ./...
