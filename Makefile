GO = go
GOFMT = gofmt
GOLANGCILINT=golangci-lint
GOSEC=gosec

export GO111MODULE = on
GO_FLAGS =

KUBECFG = kubecfg
DOCKER = docker
GINKGO = ginkgo -p

CONTROLLER_IMAGE = docker.io/bitnami/sealed-secrets-controller:latest
INSECURE_REGISTRY = false # useful for local registry
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

GOOS = $(shell go env GOOS)
GOARCH = $(shell go env GOARCH)

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

define binary
$(1)-static-$(2)-$(3): $(GO_FILES)
	GOOS=$(2) GOARCH=$(3) CGO_ENABLED=0 $(GO) build -o $$@ -installsuffix cgo $(GO_FLAGS) -ldflags "$(GO_LD_FLAGS)" ./cmd/$(1)
endef

define binaries
$(call binary,controller,$1,$2)
$(call binary,kubeseal,$1,$2)
endef

$(eval $(call binaries,linux,amd64))
$(eval $(call binaries,linux,arm64))
$(eval $(call binaries,linux,arm))
$(eval $(call binaries,darwin,amd64))
$(eval $(call binary,kubeseal,windows,amd64))

controller-static: controller-static-$(GOOS)-$(GOARCH)
	cp $< $@

kubeseal-static: kubeseal-static-$(GOOS)-$(GOARCH)
	cp $< $@


define controllerimage
controller.image.$(1)-$(2): Dockerfile controller-static-$(1)-$(2)
	mkdir -p dist/controller_$(1)_$(2)
	cp controller-static-$(1)-$(2) dist/controller_$(1)_$(2)/controller
	$(DOCKER) build --build-arg TARGETARCH=$(2) -t $(CONTROLLER_IMAGE)-$(1)-$(2) .
	@echo $(CONTROLLER_IMAGE)-$(1)-$(2) >$$@.tmp
	@mv $$@.tmp $$@
endef

$(eval $(call controllerimage,linux,amd64))
$(eval $(call controllerimage,linux,arm64))
$(eval $(call controllerimage,linux,arm))

%.yaml: %.jsonnet
	$(KUBECFG) show -V CONTROLLER_IMAGE=$(CONTROLLER_IMAGE) -V IMAGE_PULL_POLICY=$(IMAGE_PULL_POLICY) -o yaml $< > $@.tmp
	mv $@.tmp $@

controller.yaml: controller.jsonnet controller-norbac.jsonnet schema-v1alpha1.yaml kube-fixes.libsonnet

controller-norbac.yaml: controller-norbac.jsonnet schema-v1alpha1.yaml kube-fixes.libsonnet

controller-podmonitor.yaml: controller.jsonnet controller-norbac.jsonnet schema-v1alpha1.yaml kube-fixes.libsonnet

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

lint:
	 $(GOLANGCILINT) run --enable goimports --timeout=5m

lint-gosec:
	 $(GOSEC) -r --severity medium

clean:
	$(RM) ./controller ./kubeseal
	$(RM) *-static*
	$(RM) controller*.yaml
	$(RM) controller.image*

.PHONY: all kubeseal controller test clean vet fmt lint-gosec
