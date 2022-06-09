# Kubeapps apprepository-controller Developer Guide

The `apprepository-controller` is a Kubernetes controller for managing Helm chart repositories added to Kubeapps.

An AppRepository resource looks like this:

```yaml
apiVersion: v1
items:
apiVersion: kubeapps.com/v1alpha1
kind: AppRepository
metadata:
  name: bitnami
spec:
  url: https://charts.bitnami.com/incubator
  type: helm
```

This controller will monitor resources of the above type and create [Kubernetes CronJobs](https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/) to schedule the repository to be synced to the database. This is a component of Kubeapps and is intended to be used with it.

Based off the [Kubernetes Sample Controller](https://github.com/kubernetes/sample-controller).

## Prerequisites

- [Git](https://git-scm.com/)
- [Make](https://www.gnu.org/software/make/)
- [Go programming language](https://golang.org/dl/)
- [Docker CE](https://www.docker.com/community-edition)
- [Kubernetes cluster (v1.8+)](https://kubernetes.io/docs/setup/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
- [Telepresence](https://telepresence.io)

_Telepresence is not a hard requirement, but is recommended for a better developer experience_

## Download the kubeapps source code

```bash
git clone --recurse-submodules https://github.com/vmware-tanzu/kubeapps $KUBEAPPS_DIR
```

The `apprepository-controller` sources are located under the `cmd/apprepository-controller/` directory of the repository.

```bash
cd $KUBEAPPS_DIR/cmd/apprepository-controller
```

### Install Kubeapps in your cluster

Kubeapps is a Kubernetes-native application. To develop and test Kubeapps components we need a Kubernetes cluster with Kubeapps already installed. Follow the [Kubeapps installation guide](https://github.com/vmware-tanzu/kubeapps/blob/main/chart/kubeapps/README.md) to install Kubeapps in your cluster.

### Building `apprepository-controller` binary

```bash
go build
```

This builds the `apprepository-controller` binary in the working directory.

### Running in development

Before running the `apprepository-controller` binary on the development host we should stop the existing controller that is running in the development cluster. The best way to do this is to scale the number of replicas of the `apprepository-controller` deployment to `0`.

```bash
kubectl -n kubeapps scale deployment kubeapps-internal-apprepository-controller --replicas=0
```

> **NOTE** Remember to scale the deployment back to `1` replica when you are done

You can now run the `apprepository-controller` binary on the developer host with:

```bash
./apprepository-controller --repo-sync-image=docker.io/kubeapps/asset-syncer:myver --kubeconfig ~/.kube/config
```

Performing application repository actions in the Kubeapps dashboard will now trigger operations in the `apprepository-controller` binary running locally on your development host.

### Running tests

To start the tests on the `apprepository-controller` run the following command:

```bash
go test
```

## Building the kubeapps/apprepository-controller Docker image

To build the `kubeapps/apprepository-controller` docker image with the docker image tag `myver`:

```bash
cd $KUBEAPPS_DIR
make IMAGE_TAG=myver kubeapps/apprepository-controller
```
