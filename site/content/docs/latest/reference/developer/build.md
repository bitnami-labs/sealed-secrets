# The Kubeapps Build Guide

This guide explains how to build Kubeapps.

## Prerequisites

- [Git](https://git-scm.com/)
- [Make](https://www.gnu.org/software/make/)
- [Go programming language](https://golang.org/)
- [kubecfg](https://github.com/ksonnet/kubecfg)
- [Docker CE](https://www.docker.com/community-edition)

## Download kubeapps source code

```bash
git clone --recurse-submodules https://github.com/vmware-tanzu/kubeapps $KUBEAPPS_DIR
cd $KUBEAPPS_DIR
```

## Build kubeapps

Kubeapps consists of a number of in-cluster components. To build all these components in one go:

```bash
make IMAGE_TAG=myver all
```

Or if you wish to build specific component(s):

```bash
# to build the kubeapps binary
make IMAGE_TAG=myver kubeapps

# to build the kubeapps/dashboard docker image
make IMAGE_TAG=myver kubeapps/dashboard

# to build the kubeapps/apprepository-controller docker image
make IMAGE_TAG=myver kubeapps/apprepository-controller
```

## Running tests

To test all the components:

```bash
make test
```

Or if you wish to test specific component(s):

```bash
# to test the kubeapps binary
make test-kubeapps

# to test kubeapps/dashboard
make test-dashboard

# to test the cmd/apprepository-controller package
make test-apprepository-controller
```
