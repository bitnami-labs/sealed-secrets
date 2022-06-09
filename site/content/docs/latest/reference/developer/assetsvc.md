# Kubeapps assetsvc Developer Guide

The `assetsvc` component is a micro-service that creates an API endpoint for accessing the metadata for charts in Helm chart repositories that's populated in a Postgresql server.

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

The `assetsvc` sources are located under the `cmd/assetsvc/` directory.

### Install Kubeapps in your cluster

Kubeapps is a Kubernetes-native application. To develop and test Kubeapps components we need a Kubernetes cluster with Kubeapps already installed. Follow the [Kubeapps installation guide](https://github.com/vmware-tanzu/kubeapps/blob/main/chart/kubeapps/README.md) to install Kubeapps in your cluster.

### Building the `assetsvc` image

```bash
cd $KUBEAPPS_DIR
make kubeapps/assetsvc
```

This builds the `assetsvc` Docker image.

### Running in development

#### Option 1: Using Telepresence (recommended)

```bash
telepresence --swap-deployment kubeapps-internal-assetsvc --namespace kubeapps --expose 8080:8080 --docker-run --rm -ti kubeapps/assetsvc /assetsvc --database-user=postgres --database-url=kubeapps-postgresql:5432 --database-name=assets
```

Note that the assetsvc should be rebuilt for new changes to take effect.

#### Option 2: Replacing the image in the assetsvc Deployment

Note: By default, Kubeapps will try to fetch the latest version of the image so in order to make this workflow work in Minikube you will need to update the imagePullPolicy first:

```bash
kubectl patch deployment kubeapps-internal-assetsvc -n kubeapps --type=json -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/imagePullPolicy", "value": "IfNotPresent"}]'
```

```bash
kubectl set image -n kubeapps deployment kubeapps-internal-assetsvc assetsvc=kubeapps/assetsvc:latest
```

For further redeploys you can change the version to deploy a different tag or rebuild the same image and restart the pod running:

```bash
kubectl delete pod -n kubeapps -l app=kubeapps-internal-assetsvc
```

Note: If you using a cloud provider to develop the service you will need to retag the image and push it to a public registry.

### Running tests

You can run the assetsvc tests along with the tests for the Kubeapps project:

```bash
go test -v ./...
```
