# How to set up the environment using the provided makefile targets

The main file is [Makefile](https://github.com/vmware-tanzu/kubeapps/blob/main/Makefile), which will compile and prepare the production assets for then generating a set of Docker images. It is the starting point when you want to build the Kubeapps different components.

For setting up the environment for running Kubeapps, we also provide (as is) makefile targets for:

- Creating a multicluster environment with Kind ([cluster-kind.mk](https://github.com/vmware-tanzu/kubeapps/blob/main/script/makefiles/cluster-kind.mk))
- Deploying and configuring the components for getting Kubeapps running with OIDC login using Dex ([deploy-dev.mk](https://github.com/vmware-tanzu/kubeapps/blob/main/script/makefiles/deploy-dev.mk)).

> Disclaimer: these files are not being actively maintained, as they are solely intended for helping Kubeapp developers to set up the environment. If you are a contributor and you are having troubles, please feel free to [open an issue](https://github.com/vmware-tanzu/kubeapps/issues/new).

## Makefile for generating images

### Commands

Find below a list of the most used commands:

```bash
make # will make all the kubeapps images
make kubeapps/dashboard
make kubeapps/apprepository-controller
make kubeapps/kubeops
make kubeapps/assetsvc
make kubeapps/asset-syncer
```

> You can set the image tag manually: `IMAGE_TAG=myTag make`

## Makefile for setting up the environment

### Prerequisites

- Install `mkcert`; you can get it from the [official repository](https://github.com/FiloSottile/mkcert/releases).
- Get the Kind network IP and replace it when necessary.

  - Retrieve the node's IP address on the kind bridge network so it can be used by Dex:
    It is a requirement to discover the nodes IP address on the bridge network so that Dex can be reached both inside and outside the cluster at the same address.
    You can get this IP by inspecting the kind network (`docker network inspect kind`) and setting the value as **the next available IP on that network**
    (if you don't already have any kind clusters launched, this will be the first address after the gateway, ie. something like 172.x.0.2).

        * Another way to do so is to start the environment with `make cluster-kind` and manually verify the IP address by running `kubectl --namespace=kube-system get pods -o wide | grep kube-apiserver-kubeapps-control-plane  | awk '{print $6}'`, but you will need to re-create the cluster after you've updated the config files (below) by running `make delete-cluster-kind`, as some of these files (the apiserver-config ones) are config for the cluster apiserver itself, which has to know where to find dex.

  - Then, replace `172.18.0.2` with the previous IP the following files:
    - [script/makefiles/deploy-dev.mk](https://github.com/vmware-tanzu/kubeapps/blob/mainscript/makefiles/deploy-dev.mk)
    - [kubeapps-local-dev-additional-apiserver-config.yaml](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/reference/manifests/kubeapps-local-dev-additional-apiserver-config.yaml)
    - [kubeapps-local-dev-additional-kind-cluster.yaml](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/reference/manifests/kubeapps-local-dev-additional-kind-cluster.yaml)
    - [kubeapps-local-dev-apiserver-config.yaml](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/reference/manifests/kubeapps-local-dev-apiserver-config.yaml)
    - [kubeapps-local-dev-auth-proxy-values.yaml](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/reference/manifests/kubeapps-local-dev-auth-proxy-values.yaml)
    - [kubeapps-local-dev-dex-values.yaml](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/reference/manifests/kubeapps-local-dev-dex-values.yaml)

### Commands

```bash
# Create two cluster with RBAC and Nginx Ingress controller
# and configure the kube apiserver with the oidc flags
make multi-cluster-kind

# Install dex (identity service using OIDC),
# install openldap and add default users,
# generate certs for tls,
# and deploy kubeapps with the proper configuration
make deploy-dev
```
