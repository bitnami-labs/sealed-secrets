GO = go
GOTESTSUM = gotestsum
GOFMT = gofmt
GOLANGCILINT=golangci-lint -vv
GOSEC=gosec

export GO111MODULE = on
GO_FLAGS =

KUBECFG = kubecfg
DOCKER = docker
GINKGO = ginkgo -p
CONTROLLER_GEN ?= controller-gen

REGISTRY ?= docker.io
CONTROLLER_IMAGE = $(REGISTRY)/bitnami/sealed-secrets-controller:latest
KUBESEAL_IMAGE = $(REGISTRY)/bitnami/sealed-secrets-kubeseal:latest
INSECURE_REGISTRY = false # useful for local registry
IMAGE_PULL_POLICY =
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

manifests:
	$(CONTROLLER_GEN) crd:generateEmbeddedObjectMeta=true paths="./pkg/apis/..." output:stdout | tail -n +2 > helm/sealed-secrets/crds/bitnami.com_sealedsecrets.yaml
	yq '.spec.versions[0].schema' < helm/sealed-secrets/crds/bitnami.com_sealedsecrets.yaml > schema-v1alpha1.yaml

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


define image
$(1).image.$(3)-$(4): docker/$(1).Dockerfile $(1)-static-$(3)-$(4)
	mkdir -p dist/$(1)_$(3)_$(4)
	cp $(1)-static-$(3)-$(4) dist/$(1)_$(3)_$(4)/$(1)
	$(DOCKER) build --build-arg TARGETARCH=$(4) -t $(2)-$(3)-$(4) -f docker/$(1).Dockerfile .
	@echo $(2)-$(3)-$(4) >$$@.tmp
	@mv $$@.tmp $$@
endef

define images
$(call image,controller,${CONTROLLER_IMAGE},$1,$2)
$(call image,kubeseal,${KUBESEAL_IMAGE},$1,$2)
endef

$(eval $(call images,linux,amd64))
$(eval $(call images,linux,arm64))
$(eval $(call images,linux,arm))

%.yaml: %.jsonnet
	$(KUBECFG) show -V CONTROLLER_IMAGE=$(CONTROLLER_IMAGE) -V IMAGE_PULL_POLICY=$(IMAGE_PULL_POLICY) -o yaml $< > $@.tmp
	mv $@.tmp $@

controller.yaml: controller.jsonnet controller-norbac.jsonnet schema-v1alpha1.yaml kube-fixes.libsonnet

controller-norbac.yaml: controller-norbac.jsonnet schema-v1alpha1.yaml kube-fixes.libsonnet

controller-podmonitor.yaml: controller.jsonnet controller-norbac.jsonnet schema-v1alpha1.yaml kube-fixes.libsonnet

test:
	$(GOTESTSUM) $(GO_FLAGS) --junitfile report.xml --format testname -- "-coverprofile=coverage.out" $(GO_PACKAGES)

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
	 $(GOSEC) -r --severity low

clean:
	$(RM) ./controller ./kubeseal
	$(RM) *-static*
	$(RM) controller*.yaml
	$(RM) controller.image*

check-k8s:
	scripts/check-k8s

push-controller: clean check-k8s controller.image.$(OS)-$(ARCH)
	docker tag $(CONTROLLER_IMAGE)-$(OS)-$(ARCH) $(CONTROLLER_IMAGE)
ifeq ($(REGISTRY),docker.io)
  echo "Skip push: docker.io registry means minikube"
else
	docker push $(CONTROLLER_IMAGE)
endif

apply-controller-manifests: clean check-k8s controller.yaml
	kubectl apply -f controller.yaml
	kubectl rollout status deployment sealed-secrets-controller -n kube-system

controller-tests: test push-controller apply-controller-manifests clean integrationtest

.PHONY: all kubeseal controller test clean vet fmt lint-gosec

.PHONY: controllertests check-k8s push-controller apply-controller-manifests
