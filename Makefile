GO = go
GO_FLAGS =
GOFMT = gofmt

# TODO: Simplify this once ./... ignores ./vendor
GO_PACKAGES = ./cmd/... ./api/...
GO_FILES := $(shell find $(shell $(GO) list -f '{{.Dir}}' $(GO_PACKAGES)) -name \*.go)

all: controller ksonnet-seal

controller: $(GO_FILES)
	$(GO) build -o $@ $(GO_FLAGS) ./cmd/controller/...

ksonnet-seal: $(GO_FILES)
	$(GO) build -o $@ $(GO_FLAGS) ./cmd/ksonnet-seal/...

test:
	$(GO) test $(GO_FLAGS) $(GO_PACKAGES)

vet:
	$(GO) vet $(GO_FLAGS) $(GO_PACKAGES)

fmt:
	$(GOFMT) -s -w $(GO_FILES)

clean:
	$(RM) ./controller ./ksonnet-seal

.PHONY: all test clean vet fmt
