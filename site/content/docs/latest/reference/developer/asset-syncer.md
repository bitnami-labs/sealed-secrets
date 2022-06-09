# Kubeapps asset-syncer Developer Guide

The `asset-syncer` component is a tool that scans a Helm chart repository and populates chart metadata in the database. This metadata is then served by the `assetsvc` component.

## Prerequisites

- [Git](https://git-scm.com/)
- [Make](https://www.gnu.org/software/make/)
- [Go programming language](https://golang.org/dl/)
- [Docker CE](https://www.docker.com/community-edition)
- [Kubernetes cluster (v1.8+)](https://kubernetes.io/docs/setup/). [Minikube](https://github.com/kubernetes/minikube) is recommended.
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
- [Telepresence](https://telepresence.io)

## Download the Kubeapps source code

```bash
git clone https://github.com/vmware-tanzu/kubeapps $KUBEAPPS_DIR
```

The `asset-syncer` sources are located under the `cmd/asset-syncer/` directory.

### Install Kubeapps in your cluster

Kubeapps is a Kubernetes-native application. To develop and test Kubeapps components we need a Kubernetes cluster with Kubeapps already installed. Follow the [Kubeapps installation guide](https://github.com/vmware-tanzu/kubeapps/blob/main/chart/kubeapps/README.md) to install Kubeapps in your cluster.

### Building the `asset-syncer` image

```bash
cd $KUBEAPPS_DIR
make kubeapps/asset-syncer
```

This builds the `asset-syncer` Docker image.

### Running in development

```bash
export DB_PASSWORD=$(kubectl get secret --namespace kubeapps kubeapps-db -o go-template='{{index .data "postgres-password" | base64decode}}')
telepresence --namespace kubeapps --docker-run -e DB_PASSWORD=$DB_PASSWORD --rm -ti kubeapps/asset-syncer /asset-syncer sync --database-user=postgres --database-url=kubeapps-postgresql:5432 --database-name=assets stable https://kubernetes-charts.storage.googleapis.com
```

Note that the asset-syncer should be rebuilt for new changes to take effect.

### Running tests

You can run the asset-syncer tests along with the tests for the Kubeapps project:

```bash
go test -v ./...
```
