BUILD_TIME      := $(shell date "+%F %T")
COMMIT_SHA1     := $(shell git rev-parse HEAD )

# Image URL to use all building/pushing image targets
VERSION ?= v0.1.0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
	GOBIN=$(shell go env GOPATH)/bin
else
	GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build test

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet ## Run tests.
	go test ./... -coverprofile cover.out

##@ Build
LDFLAGS=-w -s
.PHONY: build
build: fmt vet ## Build binary.
	go build ./...
	go build -ldflags "$(LDFLAGS)" -o bin/zeta-modelctl-server cmd/modelctl/main.go
	go build -ldflags "$(LDFLAGS)" -o bin/zeta-modelctl-agent cmd/node/main.go

.PHONY: run ## Run from your host.
run: fmt vet run-local

.PHONY: run-local
run-local: ## Download apiserver-boot cmd locally if necessary.
	# export KUBERNETES_SERVICE_HOST=192.168.0.45
	# export KUBERNETES_SERVICE_PORT=9443
	# export KUBERNETES_MASTER=192.168.0.45
	@nohup bin/zeta-modelctl

.PHONY: docker-build
docker-build: ## Build docker image
	@sudo docker build -f ./docker/Dockerfile.server -t centaurusai.io/zeta/modelctl-server:${VERSION} ..
	@sudo docker build -f ./docker/Dockerfile.agent -t centaurusai.io/zeta/modelctl-agent:${VERSION} ..

.PHONY: docker-push
docker-push: ## Push docker image into docker hub registry
	@sudo docker push centaurusai.io/zeta/modelctl-server:${VERSION}
	@sudo docker push centaurusai.io/zeta/modelctl-agent:${VERSION}

.PHONY: containerd-push
containerd-push: containerd-push-server containerd-push-agent

.PHONY: containerd-push-server
containerd-push-server:
	@sudo docker image save -o /tmp/centaurusai.io.zeta-modelctl-server.img centaurusai.io/zeta/modelctl-server:${VERSION}
	# @sudo crictl rmi centaurusai.io/zeta/modelctl-server:${VERSION}
	@sudo ctr -n=k8s.io image import /tmp/centaurusai.io.zeta-modelctl-server.img

.PHONY: containerd-push-agent
containerd-push-agent:
	@sudo docker image save -o /tmp/centaurusai.io.zeta-modelctl-server.img centaurusai.io/zeta/modelctl-server:${VERSION}
	# @sudo crictl rmi centaurusai.io/zeta/modelctl-server:${VERSION}
	@sudo ctr -n=k8s.io image import /tmp/centaurusai.io.zeta-modelctl-server.img

.PHONY: check
check:
	@hostname
	@cat /etc/os-release
	@pwd
	@lscpu | grep CPU\(s\)
	@free -m
	make --version
	@echo "check is done"

.PHONY: clean
clean:
	@rm -f cover.out
	@rm -f bin/zeta-modelctl
	@rm -f bin/zeta-modelctl-agent

## generate grpc code for internal apis between modelctl server and modelctl node
PROTOC_GEN_GO = $(shell pwd)/bin/protoc-gen-go
PROTOC_GEN_GO_GRPC = $(shell pwd)/bin/protoc-gen-go-grpc
PROTOC = $(HOME)/.local/bin/protoc
.PHONY: grpc-gen
grpc-gen: ## Download protc-gen locally if necessary.
	$(call go-get-tool,$(PROTOC_GEN_GO),google.golang.org/protobuf/cmd/protoc-gen-go@v1.28)
	$(call go-get-tool,$(PROTOC_GEN_GO_GRPC),google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2)
	$(call get-protoc,$(PROTOC))
	go mod vendor
	$(PROTOC) -I=./ -I=./vendor \
		--go_out=../../.. \
		--go-grpc_out=../../.. \
		--go_opt=Mk8s.io/api/core/v1/generated.proto=k8s.io/api/core/v1 \
		--go_opt=Mk8s.io/apimachinery/pkg/api/resource/generated.proto=k8s.io/apimachinery/pkg/api/resource \
		--go_opt=Mk8s.io/apimachinery/pkg/apis/meta/v1/generated.proto=k8s.io/apimachinery/pkg/apis/meta/v1 \
		--go_opt=Mk8s.io/apimachinery/pkg/runtime/generated.proto=k8s.io/apimachinery/pkg/runtime \
		--go_opt=Mk8s.io/apimachinery/pkg/runtime/schema/generated.proto=k8s.io/apimachinery/pkg/runtime/schema \
		--go_opt=Mk8s.io/apimachinery/pkg/util/intstr/generated.proto=k8s.io/apimachinery/pkg/util/intstr \
		--go_opt=MCentaurusAI/zeta/inference/apis/core/v1alpha1/generated.proto=CentaurusAI/zeta/inference/apis/core/v1alpha1 \
		./grpc/modelctl.proto
