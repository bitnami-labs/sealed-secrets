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

You need a kubernetes cluster to run the intgeration tests.

For instance:

- Start Minikube and configure your local environment to re-use the Docker daemon inside the Minikube instance:

```bash
minikube start
eval $(minikube docker-env)
```

### Run all controller tests with a single command

```bash
make K8S_CONTEXT=mytestk8s-context OS=linux ARCH=amd64 controller-tests
```

Note that:
- `K8S_CONTEXT` must be set to the name of your `kubectl` context pointing to the expected text cluster.
- `OS` & `ARCH` must match the Operating System and Architecture of your test cluster.

Optionally, you can customize the `REGISTRY` as well:

```bash
make K8S_CONTEXT=kind-mykind REGISTRY=localhost:5000 OS=linux ARCH=amd64 controller-tests
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

```bash
make OS=linux ARCH=amd64 push-controller
```

This builds the controller container image and pushes it.

Remember that the `REGISTRY` env var is only needed when using a custom registry:

```bash
make REGISTRY=localhost:5000 OS=linux ARCH=amd64 push-controller
```

#### Building & applying the controller manifests

```bash
make K8S_CONTEXT=kind-mykind REGISTRY=localhost:5000 apply-controller-manifests
```

This builds the controller K8s manifests in the working directory and deploys them.

#### Running integration tests

```bash
make integrationtest
```
