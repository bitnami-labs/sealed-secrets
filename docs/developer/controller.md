# Controller Developer Guide

The controller is in charge of keeping the current state of `SealedSecret` objects in sync with the declared desired state.

## Download the controller source code

```bash
git clone https://github.com/bitnami-labs/sealed-secrets.git $SEALED_SECRETS_DIR
```

The controller sources are located under `cmd/controller/` and use packages from the `pkg` directory.

### Building the `controller` binary

```bash
make controller
```

This builds the `controller` binary in the working directory.

### Running unit tests

To run the unit tests for `controller` binary:

```bash
make test
```

### Building the controller image

```bash
CONTROLLER_IMAGE="bitnami/sealed-secrets-controller:development"
make CONTROLLER_IMAGE=$CONTROLLER_IMAGE controller.image.linux-amd64
docker tag $CONTROLLER_IMAGE-linux-amd64 $CONTROLLER_IMAGE
```

This builds the controller container image.

### Building the controller manifests

```bash
make CONTROLLER_IMAGE=$CONTROLLER_IMAGE IMAGE_PULL_POLICY=Never controller.yaml
```

This builds the controller K8s manifests in the working directory.

### Running integration tests

To run the integration tests:

- Start Minikube and configure your local environment to re-use the Docker daemon inside the Minikube instance:

```bash
minikube start
eval $(minikube docker-env)
```

- [Build the controller container image](#building-the-controller-image).
- [Build the controller manifests](#building-the-controller-manifests).

- Deploy the Sealed Secrets CRD and controller to your Minikube cluster:

```bash
kubectl apply -f controller.yaml
```

- Clean the environment and run the integration tests:

```bash
make clean
make integrationtest
```
