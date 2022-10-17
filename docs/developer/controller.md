# Controller Developer Guide

The controller is in charge of keeping the current state of `SealedSecret` objects in sync with the declared desired state.

The controller exposes an API defined using the Swagger or OpenAPI v3 specification. You can download the definition from the link below:

- [swagger.yml](swagger.yml)

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Download the controller source code](#download-the-controller-source-code)
  - [Setup a kubernetes cluster to run the tests](#setup-a-kubernetes-cluster-to-run-the-tests)
  - [Run all controller tests with a single command](#run-all-controller-tests-with-a-single-command)
  - [Run tests step by step](#run-tests-step-by-step)
    - [Building the `controller` binary](#building-the-controller-binary)
    - [Running unit tests](#running-unit-tests)
    - [Push the controller image](#push-the-controller-image)
    - [Building & applying the controller manifests](#building--applying-the-controller-manifests)
    - [Running integration tests](#running-integration-tests)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Download the controller source code

```bash
git clone https://github.com/bitnami-labs/sealed-secrets.git $SEALED_SECRETS_DIR
```

The controller sources are located under `cmd/controller/` and use packages from the `pkg` directory.


### Setup a kubernetes cluster to run the tests

You need a kubernetes cluster to run the integration tests.

For instance:

When using a local minikube, configure your local environment to re-use the local Docker daemon:

```bash
minikube start
eval $(minikube docker-env)
```

If you use `kind` instead, you can setup a local companion image registry and allow kind to access it.

Sample to run a registry locally:
```bash
export LOCAL_REGISTRY_PORT='5000'
export LOCAL_REGISTRY_NAME='kind-registry'
docker run --rm -d -p "127.0.0.1:${LOCAL_REGISTRY_PORT}:5000" --name "${LOCAL_REGISTRY_NAME}" registry:2
```

Then to have launch `kind` with access to that registry:
```bash
cat <<EOF | kind create cluster --name "${CLUSTER_NAME}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${LOCAL_REGISTRY_PORT}"]
    endpoint = ["http://${LOCAL_REGISTRY_NAME}:5000"]
EOF
docker network connect "kind" "${LOCAL_REGISTRY_NAME}"
```

### Run all controller tests with a single command

```bash
make K8S_CONTEXT=mytestk8s-context OS=linux ARCH=amd64 controller-tests
```

Note that:
- `K8S_CONTEXT` must be set to the name of your `kubectl` context pointing to the expected text cluster.
- `OS` & `ARCH` must match the Operating System and Architecture of your test cluster.

Optionally, you can customize the `REGISTRY` as well. In fact you will need that for a kind setup with a local registry:

```bash
make K8S_CONTEXT=kind REGISTRY=localhost:5000 OS=linux ARCH=amd64 controller-tests
```

For minikube just skip the `REGISTRY` setting:
```bash
make K8S_CONTEXT=minikube OS=linux ARCH=amd64 controller-tests
```

### Run tests step by step

#### Building the `controller` binary

```bash
make controller
```

This builds the `controller` binary in the working directory.

#### Running unit tests

To run the unit tests for `controller` binary:

```bash
make test
```

#### Push the controller image

This would work with a local minikube setup to build the controller:

```bash
make K8S_CONTEXT=minikube OS=linux ARCH=amd64 push-controller
```

It will not push, as minikube accesses local docker images directly.

Remember the `REGISTRY` env var is needed when using a custom registry:

```bash
make K8S_CONTEXT=kind REGISTRY=localhost:5000 OS=linux ARCH=amd64 push-controller
```

This builds the controller container image and pushes it.

#### Building & applying the controller manifests

```bash
make K8S_CONTEXT=minikube apply-controller-manifests
```

Or for `kind`:

```bash
make K8S_CONTEXT=kind REGISTRY=localhost:5000 apply-controller-manifests
```

This builds the controller K8s manifests in the working directory and deploys them.

#### Running integration tests

```bash
make integrationtest
```
