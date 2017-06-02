GO = go
GO_FLAGS =
GOFMT = gofmt

KUBECFG = kubecfg
DOCKER = docker

CONTROLLER_IMAGE = sealed-secrets-controller:latest

# TODO: Simplify this once ./... ignores ./vendor
GO_PACKAGES = ./cmd/... ./apis/...
GO_FILES := $(shell find $(shell $(GO) list -f '{{.Dir}}' $(GO_PACKAGES)) -name \*.go)

all: controller ksonnet-seal

controller: $(GO_FILES)
	$(GO) build -o $@ $(GO_FLAGS) ./cmd/controller

ksonnet-seal: $(GO_FILES)
	$(GO) build -o $@ $(GO_FLAGS) ./cmd/ksonnet-seal

%-static: $(GO_FILES)
	CGO_ENABLED=0 $(GO) build -o $@ -installsuffix cgo $(GO_FLAGS) ./cmd/$*

controller.image: Dockerfile.single controller-static
	$(DOCKER) build -f Dockerfile.single -t $(CONTROLLER_IMAGE) .
	#$(DOCKER) image inspect $(CONTROLLER_IMAGE) -f '$(shell echo $(CONTROLLER_IMAGE) | cut -d: -f1)@{{.Id}}' > $@.tmp
	echo $(CONTROLLER_IMAGE) >$@.tmp
	mv $@.tmp $@

controller.yaml: controller.jsonnet controller.image
	$(KUBECFG) show -o yaml $< > $@.tmp
	mv $@.tmp $@

test:
	$(GO) test $(GO_FLAGS) $(GO_PACKAGES)

vet:
	$(GO) vet $(GO_FLAGS) $(GO_PACKAGES)

fmt:
	$(GOFMT) -s -w $(GO_FILES)

clean:
	$(RM) ./controller ./ksonnet-seal

.PHONY: all test clean vet fmt
