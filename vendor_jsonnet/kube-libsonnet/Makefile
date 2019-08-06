# Originally taken from https://github.com/bitnami-labs/kube-manifests/,
# trimmed down to only run lib testing.
#
# Provides 'test' target.  Uses docker.

UID := $(shell id -u)
GID := $(shell id -g)

# Eg: if you need sudo, run with DOCKER_PREFIX=sudo
DOCKER_PREFIX =

DOCKER = $(DOCKER_PREFIX) docker
DOCKER_BUILD = $(DOCKER) build --build-arg http_proxy=$(http_proxy)
DOCKER_RUN = $(DOCKER) run --rm --network=host -u $(UID):$(GID) \
 -v $(CURDIR):$(CURDIR) -w $(CURDIR) \
 -v $(HOME)/.kube/config:/kubeconfig \
 -v $(HOME)/.kube/cache:/home/user/.kube/cache \
 -e TERM=$(TERM) -e KUBECONFIG=/kubeconfig

all: tests

docker-kube-manifests: tests/Dockerfile
	if [ -z "$(shell $(DOCKER) images -q kube-manifests)" ]; then \
	  $(DOCKER_BUILD) -t kube-manifests tests; \
	fi

tests: docker-kube-manifests
	$(DOCKER_RUN) kube-manifests make -C tests


.PHONY: all build test docker-kube-manifests
