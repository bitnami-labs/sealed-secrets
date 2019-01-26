GO = go
GO_FLAGS =
GOFMT = gofmt
GORELEASER = goreleaser

KUBECFG = kubecfg -U https://github.com/bitnami-labs/kube-libsonnet/raw/52ba963ca44f7a4960aeae9ee0fbee44726e481f
DOCKER = docker
GINKGO = ginkgo -p

# + is not a permitted character in image tags and '+dirty' is not passed in to goreleaser breaking snapshots
CONTROLLER_IMAGE = $(subst +dirty,,quay.io/bitnami-labs/sealed-secrets-controller:$(VERSION))
KUBECONFIG ?= $(HOME)/.kube/config

# TODO: Simplify this once ./... ignores ./vendor
GO_PACKAGES = ./cmd/... ./pkg/...
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
	$(KUBECFG) show -V CONTROLLER_IMAGE=$(CONTROLLER_IMAGE) -o yaml $< > $@.tmp
	mv $@.tmp $@

controller.yaml: controller.jsonnet controller-norbac.jsonnet

controller-norbac.yaml: controller-norbac.jsonnet

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
	$(RM) -r dist/

release: clean ## Generate a release, but don't publish to GitHub.
	$(GORELEASER) --skip-validate --skip-publish

publish: clean ## Generate a release, and publish to GitHub.
	$(GORELEASER)

snapshot: clean ## Generate a snapshot release.
	$(GORELEASER) --snapshot --skip-validate --skip-publish

.PHONY: all kubeseal controller test clean vet fmt
