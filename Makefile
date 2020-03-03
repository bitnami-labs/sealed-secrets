GO = go
GOFMT = gofmt

export GO111MODULE = on
GO_FLAGS = -mod=vendor

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

CONTROLLER_IMAGE_PER_ARCH =
PUSHED_CONTROLLER_IMAGE_PER_ARCH =

define controllerimage
controller.image.$(1)-$(2): docker/Dockerfile controller-static-$(1)-$(2)
	cp controller-static-$(1)-$(2) docker/controller
	$(DOCKER) build -t $(CONTROLLER_IMAGE)-$(1)-$(2) docker/
	@echo $(CONTROLLER_IMAGE)-$(1)-$(2) >$$@.tmp
	@mv $$@.tmp $$@

pushed.controller.image.$(1)-$(2): controller.image.$(1)-$(2)
	$(DOCKER) push $(CONTROLLER_IMAGE)-$(1)-$(2)
	$(DOCKER) inspect --format='{{index .RepoDigests 0}}' $(CONTROLLER_IMAGE)-$(1)-$(2) >$$@.tmp
	@mv $$@.tmp $$@

CONTROLLER_IMAGE_PER_ARCH += controller.image.$1-$2
PUSHED_CONTROLLER_IMAGE_PER_ARCH += pushed.controller.image.$1-$2

endef

ARCHS = amd64 arm64 arm

$(eval $(call controllerimage,linux,amd64))
$(eval $(call controllerimage,linux,arm64))
$(eval $(call controllerimage,linux,arm))

# the only way to escape a comma in Make is via a dummy variable
comma=,

# A docker manifest is used for multiarch images.
# It requires pre-pushed images, which are pushed by the pushed.controller.image* targets
# which are not pushing the final image but only constituents of the multiarch image assembled here.
# To push this manifest (which is what end users actually reference as docker images) use push-controller-image below.
controller-manifest-%: IMAGE=$(subst $(comma),:,$(subst %,/,$*))
controller-manifest-%: $(PUSHED_CONTROLLER_IMAGE_PER_ARCH)
	@echo "composing multiarch manifest for $(IMAGE)"

	$(DOCKER) manifest create $(IMAGE) $(foreach i,$(PUSHED_CONTROLLER_IMAGE_PER_ARCH),$(shell cat $(i)))
	$(foreach i,$(ARCHS),$(DOCKER) manifest annotate --arch $(i) $(IMAGE) $(shell cat pushed.controller.image.linux-$(i));)
	@echo $(IMAGE) >$@.tmp
	@mv $@.tmp $@

# push-controller-image pushes the docker image of the sealed-secrets controller to the docker
# registry with an image name defined in the CONTROLLER_IMAGE variable (which includes a tag).
#
# The controller image is a multi-arch (v2) docker manifest that points to one docker image per supported
# architecture (currently only for linux; see the $(ARCHS) variable for a list of supported cpu arch).
push-controller-image: controller.image
	$(DOCKER) manifest push -p $(CONTROLLER_IMAGE) >$@.tmp
	@mv $@.tmp $@
	@echo pushed: $(CONTROLLER_IMAGE)@$$(cat $@)

# The RHS is a bit convoluted: the intention is to allow to execute this rule
# multiple times with different values of the CONTROLLER_IMAGE variable while avoiding unnecessary
# work (such as rebuilding the underlying docker images for each architecture).
controller.image: controller-manifest-$(subst :,$(comma),$(subst /,%,$(CONTROLLER_IMAGE)))

%.yaml: %.jsonnet
	$(KUBECFG) show -V CONTROLLER_IMAGE=$(CONTROLLER_IMAGE) -V IMAGE_PULL_POLICY=$(IMAGE_PULL_POLICY) -o yaml $< > $@.tmp
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
	$(RM) *-static*
	$(RM) controller*.yaml
	$(RM) controller.image*
	$(RM) pushed.controller.image*
	$(RM) controller-manifest-*
	$(RM) push-controller-image

.PHONY: all kubeseal controller test clean vet fmt
