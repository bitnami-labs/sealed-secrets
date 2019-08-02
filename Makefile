GO = go
GOFMT = gofmt

USE_GO_MOD := $(shell echo $${USE_GO_MOD:-yes})
ifeq ($(USE_GO_MOD),yes)
export GO111MODULE = on
GO_FLAGS = -mod=vendor
else
export GO111MODULE = off
GO_FLAGS =
endif

KUBECFG = kubecfg
DOCKER = docker
GINKGO = ginkgo -p

CONTROLLER_IMAGE = quay.io/bitnami/sealed-secrets-controller:latest
IMAGE_PULL_POLICY = Always
KUBECONFIG ?= $(HOME)/.kube/config

GO_PACKAGES = ./...
GO_FILES := $(shell find $(shell $(GO) list -f '{{.Dir}}' $(GO_PACKAGES)) -name \*.go)

COMMIT = $(shell git rev-parse HEAD)
TAG = $(shell git describe --exact-match --abbrev=0 --tags '$(COMMIT)' 2> /dev/null || true)
DIRTY = $(shell git diff --shortstat 2> /dev/null | tail -n1)

# Use a tag if set, otherwise use the commit hash
ifeq ($(TAG),)
VERSION := $(COMMIT)
else
VERSION := $(TAG)
endif

# Check for changed files
ifneq ($(DIRTY),)
VERSION := $(VERSION)+dirty
endif

GO_LD_FLAGS = -X main.VERSION=$(VERSION)

all: controller kubeseal

generate: $(GO_FILES)
	$(GO) generate $(GO_PACKAGES)

controller: $(GO_FILES)
	$(GO) build -o $@ $(GO_FLAGS) -ldflags "$(GO_LD_FLAGS)" ./cmd/controller

kubeseal: $(GO_FILES)
	$(GO) build -o $@ $(GO_FLAGS) -ldflags "$(GO_LD_FLAGS)" ./cmd/kubeseal

%-static: $(GO_FILES)
	CGO_ENABLED=0 $(GO) build -o $@ -installsuffix cgo $(GO_FLAGS) -ldflags "$(GO_LD_FLAGS)" ./cmd/$*

docker/controller: controller-static
	cp $< $@

controller.image: docker/Dockerfile docker/controller
	$(DOCKER) build -t $(CONTROLLER_IMAGE) docker/
	echo $(CONTROLLER_IMAGE) >$@.tmp
	mv $@.tmp $@

%.yaml: %.jsonnet
	$(KUBECFG) show -V CONTROLLER_IMAGE=$(CONTROLLER_IMAGE) -V IMAGE_PULL_POLICY=$(IMAGE_PULL_POLICY) -o yaml $< > $@.tmp
	mv $@.tmp $@

controller.yaml: controller.jsonnet controller.image controller-norbac.jsonnet

controller-norbac.yaml: controller-norbac.jsonnet controller.image

test:
	$(GO) test $(GO_FLAGS) $(GO_PACKAGES)

integrationtest: kubeseal controller
	# Assumes a k8s cluster exists, with controller already installed
	$(GINKGO) -tags 'integration' integration -- -kubeconfig $(KUBECONFIG) -kubeseal-bin $(abspath $<) -controller-bin $(abspath $(word 2,$^))

vet:
	# known issue:
	# pkg/client/clientset/versioned/fake/clientset_generated.go:46: literal copies lock value from fakePtr
	$(GO) vet $(GO_FLAGS) -copylocks=false $(GO_PACKAGES)

fmt:
	$(GOFMT) -s -w $(GO_FILES)

clean:
	$(RM) ./controller ./kubeseal
	$(RM) *-static
	$(RM) controller*.yaml
	$(RM) docker/controller

.PHONY: all kubeseal controller test clean vet fmt
