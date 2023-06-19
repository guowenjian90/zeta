BUILD_TIME      := $(shell date "+%F %T")
COMMIT_SHA1     := $(shell git rev-parse HEAD )

# Image URL to use all building/pushing image targets
VERSION ?= v0.1.0
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
# Golang forbids converting float from Json to struct. Need allowDangerousTypes=true to unblock build.
CRD ?= "crd:maxDescLen=100,generateEmbeddedObjectMeta=true,allowDangerousTypes=true"
# CRD ?= "crd:maxDescLen=100,trivialVersions=true,preserveUnknownFields=false,generateEmbeddedObjectMeta=true,allowDangerousTypes=true"

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

## Generate code for external api resources and internal grpc protocol
.PHONY: generate
generate: manifests client-gen

.PHONY: manifests ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
manifests: code-gen-tool
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./apis/..."
	$(CONTROLLER_GEN) $(CRD) rbac:roleName=zeta-inference webhook paths="./apis/..." output:crd:artifacts:config=config/crd/bases

## Generate client-go sdk containing clientset, lister, and informer method implementations.
CLIENT_CODEGEN = $(shell pwd)/hack/client_codegen.sh
.PHONY: client-gen
client-gen: code-gen-tool
	$(CLIENT_CODEGEN) "client, lister, informer"  CentaurusAI/zeta/inference/client "CentaurusAI/zeta/inference/apis" "core:v1alpha1" --go-header-file hack/boilerplate.go.txt

## Generate protobuf for api resource model
PROTOC_GEN_GOGO = $(shell pwd)/bin/protoc-gen-gogo
.PHONY: resource-grpc-gen
resource-grpc-gen: code-gen-tool
	$(call go-get-tool,$(PROTOC_GEN_GOGO),github.com/gogo/protobuf/protoc-gen-gogo@v1.3.2)
	$(GO_TO_PROTOBUF) --go-header-file="./hack/boilerplate.go.txt" \
		--packages="CentaurusAI/zeta/inference/apis/core/v1alpha1" \
		--proto-import="./vendor" --vendor-output-base="./vendor" \
		--apimachinery-packages="+k8s.io/apimachinery/pkg/util/intstr,+k8s.io/apimachinery/pkg/api/resource,+k8s.io/apimachinery/pkg/runtime/schema,+k8s.io/apimachinery/pkg/runtime,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/apis/meta/v1beta1,k8s.io/api/core/v1"
		# --apimachinery-packages="+k8s.io/apimachinery/pkg/util/intstr,+k8s.io/apimachinery/pkg/api/resource,+k8s.io/apimachinery/pkg/runtime/schema,+k8s.io/apimachinery/pkg/runtime,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/apis/meta/v1beta1,k8s.io/apimachinery/pkg/apis/testapigroup/v1,k8s.io/api/core/v1"

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet envtest ## Run tests.
	go test ./... -coverprofile cover.out

##@ Build
LDFLAGS=-w -s
.PHONY: build
build: fmt vet ## Build binary.
	go build ./...
	go build -ldflags "$(LDFLAGS)" -o bin/zeta-inference-controller cmd/main.go

.PHONY: run ## Run from your host.
run: generate fmt vet run-local

.PHONY: run-local
run-local: 
	@nohup bin/zeta-inference-controller 

.PHONY: docker-build
docker-build: ## Build docker image
	@sudo docker build -f ./docker/Dockerfile -t centaurusai.io/zeta/inference-controller:${VERSION} .

.PHONY: docker-push
docker-push: ## Push docker image into docker hub registry
	@sudo docker push centaurusai.io/zeta/inference-controller:${VERSION}

.PHONY: containerd-push
containerd-push:
	@sudo docker image save -o /tmp/centaurusai.io.zeta-inference-controller.img centaurusai.io/zeta/inference-controller:${VERSION}
	# @sudo crictl rmi centaurusai.io/zeta/inference-controller:${VERSION}
	@sudo ctr -n=k8s.io image import /tmp/centaurusai.io.zeta-inference-controller.img

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
	@rm -f bin/zeta-inference-controller

helm: manifests kustomize ## Sync the CRDs into the Helm chart
	rm -r ../helm-chart/zeta-inference/crds/
	cp -r config/crd/bases/ ../helm-chart/zeta-inference/crds/

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	($(KUSTOMIZE) build config/crd | kubectl create -f -) || ($(KUSTOMIZE) build config/crd | kubectl replace -f -)


.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image zeta/inference-controller=${IMG}
	($(KUSTOMIZE) build config/default | kubectl create -f -) || ($(KUSTOMIZE) build config/default | kubectl replace -f -)

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -

KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v3@v3.8.7)

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
OPENAPI_GEN = $(shell pwd)/bin/openapi-gen
CLIENT_GEN = $(shell pwd)/bin/client-gen		## use it to generate clientset
LISTER_GEN = $(shell pwd)/bin/lister-gen		## use it to generate lister watch
INFORMER_GEN = $(shell pwd)/bin/informer-gen    ## use it to generate informer info
GO_TO_PROTOBUF = $(shell pwd)/bin/go-to-protobuf
.PHONY: code-gen-tool
code-gen-tool: ## Download client-genl, lister-gen and informer-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.9.0)
	$(call go-get-tool,$(CLIENT_GEN),k8s.io/code-generator/cmd/client-gen@v0.26.1)
	$(call go-get-tool,$(LISTER_GEN),k8s.io/code-generator/cmd/lister-gen@v0.26.1)
	$(call go-get-tool,$(INFORMER_GEN),k8s.io/code-generator/cmd/informer-gen@v0.26.1)
	$(call go-get-tool,$(GO_TO_PROTOBUF),k8s.io/code-generator/cmd/go-to-protobuf@v0.26.1)
	$(call go-get-tool,$(OPENAPI_GEN),k8s.io/kube-openapi/cmd/openapi-gen@v0.0.0-20211115234752-e816edb12b65)

ENVTEST = $(shell pwd)/bin/setup-envtest
.PHONY: envtest
envtest: ## Download envtest-setup locally if necessary.
	$(call go-get-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest@latest)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

define get-protoc
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
PB_REL="https://github.com/protocolbuffers/protobuf/releases" ;\
curl -LO $$PB_REL/download/v3.12.1/protoc-3.12.1-linux-x86_64.zip ;\
echo "get protoc" ;\
unzip $$TMP_DIR/protoc-3.12.1-linux-x86_64.zip -d $$HOME/.local ;\
rm -rf $$TMP_DIR ;\
}
endef
