# Developer Guide

**Table of Contents**

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Prerequisites](#prerequisites)
- [The Sealed Secrets Components](#the-sealed-secrets-components)
  - [Controller](#controller)
  - [Kubeseal](#kubeseal)
- [git-hooks](#git-hooks)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Prerequisites

To be able to develop on this project, you need to have the following tools installed:

- [Git](https://git-scm.com/)
- [Make](https://www.gnu.org/software/make/)
- [Go programming language](https://golang.org/dl/)
- [Docker CE](https://www.docker.com/community-edition)
- [Kubernetes cluster (v1.16+)](https://kubernetes.io/docs/setup/). [Minikube](https://github.com/kubernetes/minikube) is recommended.
- [Kubecfg](https://github.com/bitnami/kubecfg)
- [Ginkgo](https://onsi.github.io/ginkgo/)
- [git-hooks](https://github.com/git-hooks/git-hooks)
- [doctoc](https://github.com/thlorenz/doctoc)

## The Sealed Secrets Components

Sealed Secrets is composed of three parts:

- A [custom resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources) named `SealedSecret`
- A cluster-side controller / operator that manages the `SealedSecret` objects
- A client-side utility: kubeseal

### Controller

The controller is in charge of keeping the current state of `SealedSecret` objects in sync with the declared desired state.

Please refer to the [Sealed Secrets Controller](controller.md) for the developer setup.

### Kubeseal

The `kubeseal` utility uses asymmetric crypto to encrypt secrets that only the controller can decrypt.

Please refer to the [Kubeseal Developer Guide](kubeseal.md) for the developer setup.

## git-hooks

To avoid easily detectable issues and prevent them from reaching main, some validations have been implemented via [git hooks](https://git-scm.com/book/en/v2/Customizing-Git-Git-Hooks). To have those hooks committed in the repository you need to install a third party tool `git-hooks` (check [prerequisites](#prerequisites)), because the hooks provided by Git are stored in the `.git` directory that is not included as part of the repositories.

Currently, there's a single hook at pre-commit level. This hook ensures the Table of Contents (TOC) is updated using `doctoc` (check [prerequisites](#prerequisites)) in every `.md` and `.txt` file that uses this tool.

Configure git-hooks for this specific repository by running `git hooks install`. You can check with the following command if everything was configured properly:

```console
$ git hooks list
Git hooks ARE installed in this repository.
project hooks
  pre-commit
    - doc-toc

Contrib hooks
```
