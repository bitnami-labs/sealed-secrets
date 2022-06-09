# Offline Installation of Kubeapps

Since the version 1.10.1 of Kubeapps (Chart version 3.7.0), it's possible to successfully install Kubeapps in an offline environment. To be able to able to install Kubeapps without Internet connection, it's necessary to:

- Pre-download the Kubeapps chart.
- Mirror Kubeapps images so they are accessible within the cluster.
- [Optional] Have one or more offline Package Repositories.

## 1. Download the Kubeapps chart

First, download the tarball containing the Kubeapps chart from the publicly available repository maintained by Bitnami. Note that Internet connection is necessary at this point:

```bash
helm pull --untar https://charts.bitnami.com/bitnami/kubeapps-4.0.4.tgz
helm dep update ./kubeapps
```

> Latest version of this chart available at the Bitnami Chart [Repository](https://github.com/bitnami/charts/blob/master/bitnami/kubeapps/Chart.yaml#L3)

## 2. Mirror Kubeapps images

In order to be able to install Kubeapps, it's necessary to either have a copy of all the images that Kubeapps requires in each node of the cluster or push these images to an internal Docker registry that Kubernetes can access. You can obtain the list of images by checking the `values.yaml` of the chart. For example:

```yaml
registry: docker.io
repository: bitnami/nginx
tag: 1.19.2-debian-10-r32
```

> This list includes but is not limited to: `bitnami/kubeapps-apprepository-controller`, `bitnami/kubeapps-asset-syncer`,`bitnami/kubeapps-assetsvc`, `bitnami/kubeapps-dashboard`, `bitnami/kubeapps-kubeops`,`bitnami/kubeapps-pinniped-proxy`, `bitnami/kubeapps-apis`, `bitnami/nginx`, `bitnami/oauth2-proxy`, `bitnami/postgresql`.

For simplicity, in this guide, we use a single-node cluster created with [Kubernetes in Docker (`kind`)](https://github.com/kubernetes-sigs/kind). In this environment, as the images have to be preloaded, we first have to pull the images (`docker pull`) and next load them into the cluster (`kind load docker-image`):

```bash
docker pull bitnami/nginx:1.19.2-debian-10-r32
kind load docker-image bitnami/nginx:1.19.2-debian-10-r32
```

In case you are using a private Docker registry, you will need to re-tag the images and push them:

```bash
docker pull bitnami/nginx:1.19.2-debian-10-r32
docker tag bitnami/nginx:1.19.2-debian-10-r32 REPO_URL/bitnami/nginx:1.19.2-debian-10-r32
docker push REPO_URL/bitnami/nginx:1.19.2-debian-10-r32
```

You will need to follow a similar process for every image present in the values file.

## 3. [Optional] Prepare an offline Package Repository

By default, Kubeapps install the `bitnami` Package Repository. Since, in order to sync that repository, it's necessary to have Internet connection, you will need to mirror it or create your own repository (e.g. using Harbor) and configure it when installing Kubeapps.

For more information about how to create a private repository, follow this [guide](./private-app-repository.md).

## 4. Install Kubeapps

Now that you have everything pre-loaded in your cluster, it's possible to install Kubeapps using the chart directory from the first step:

**NOTE**: If during step 2), you were using a private docker registry, it's necessary to modify the global value used for the registry. This can be set by specifying `--set global.imageRegistry=REPO_URL`.
If this registry, additionally, needs an ImagePullSecret, specify it with `--set global.imagePullSecrets[0]=SECRET_NAME`.

```bash
helm install kubeapps ./kubeapps [OPTIONS]
```
