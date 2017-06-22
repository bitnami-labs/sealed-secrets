GO = go
GO_FLAGS =
GOFMT = gofmt

KUBECFG = kubecfg
DOCKER = docker

DOCKER_USE_SHA = 0
CONTROLLER_IMAGE = sealed-secrets-controller:latest

# TODO: Simplify this once ./... ignores ./vendor
GO_PACKAGES = ./cmd/... ./apis/...
GO_FILES := $(shell find $(shell $(GO) list -f '{{.Dir}}' $(GO_PACKAGES)) -name \*.go)

all: controller kubeseal

controller: $(GO_FILES)
	$(GO) build -o $@ $(GO_FLAGS) ./cmd/controller

kubeseal: $(GO_FILES)
	$(GO) build -o $@ $(GO_FLAGS) ./cmd/kubeseal

%-static: $(GO_FILES)
	CGO_ENABLED=0 $(GO) build -o $@ -installsuffix cgo $(GO_FLAGS) ./cmd/$*

docker/controller: controller-static
	cp $< $@

controller.image: docker/Dockerfile docker/controller
	$(DOCKER) build -t $(CONTROLLER_IMAGE) docker/
ifeq ($(DOCKER_USE_SHA),1)
	$(DOCKER) image inspect $(CONTROLLER_IMAGE) -f '$(shell echo $(CONTROLLER_IMAGE) | cut -d: -f1)@{{.Id}}' > $@.tmp
else
	echo $(CONTROLLER_IMAGE) >$@.tmp
endif
	mv $@.tmp $@

%.yaml: %.jsonnet
	$(KUBECFG) show -o yaml $< > $@.tmp
	mv $@.tmp $@

controller.yaml: controller.jsonnet controller.image controller-norbac.jsonnet

controller-norbac.yaml: controller-norbac.jsonnet controller.image

test:
	$(GO) test $(GO_FLAGS) $(GO_PACKAGES)

vet:
	$(GO) vet $(GO_FLAGS) $(GO_PACKAGES)

fmt:
	$(GOFMT) -s -w $(GO_FILES)

clean:
	$(RM) ./controller ./kubeseal
	$(RM) *-static
	$(RM) controller*.yaml
	$(RM) docker/controller

.PHONY: all test clean vet fmt
