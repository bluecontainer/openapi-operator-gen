# Image URL to use all building/pushing image targets
IMG ?= controller:latest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

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

.PHONY: build
build: fmt vet ## Build manager binary.
	go build -o bin/manager cmd/manager/main.go

.PHONY: run
run: fmt vet ## Run a controller from your host.
	go run ./cmd/manager/main.go

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}

##@ Deployment

.PHONY: install
install: ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	kubectl apply -f config/crd/bases/

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	kubectl delete -f config/crd/bases/

.PHONY: generate
generate: ## Generate code (deep copy, etc.)
	go generate ./...
	controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
	controller-gen crd paths="./..." output:crd:artifacts:config=config/crd/bases

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.14.0)

define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(shell pwd)/bin go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef
