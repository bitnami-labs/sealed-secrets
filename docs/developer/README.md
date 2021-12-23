# Developer Guide

## Prerequisites

To be able to develop on this project, you need to have the following tools installed:

- [Git](https://git-scm.com/)
- [Make](https://www.gnu.org/software/make/)
- [Go programming language](https://golang.org/dl/)
- [Docker CE](https://www.docker.com/community-edition)
- [Kubernetes cluster (v1.16+)](https://kubernetes.io/docs/setup/). [Minikube](https://github.com/kubernetes/minikube) is recommended.
- [Kubecfg](https://github.com/bitnami/kubecfg)
- [Ginkgo](https://onsi.github.io/ginkgo/)

## The Sealed Secrets Components

Sealed Secrets is composed of three parts:

- A [custom resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) named `SealedSecret`
- A cluster-side controller / operator that manages the `SealedSecret` objects
- A client-side utility: kubeseal

### Controller

The controller is in charge of keeping the current state of `SealedSecret` objects in sync with the declared desired state.

Please refer to the [Sealed Secrets Controller](controller.md) for the developer setup.

### Kubeseal

The `kubeseal` utility uses asymmetric crypto to encrypt secrets that only the controller can decrypt.

Please refer to the [Kubeseal Developer Guide](kubeseal.md) for the developer setup.
