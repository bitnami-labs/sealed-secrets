# Get Started with Sealed Secrets

## Table of Contents

1. [Introduction](#introduction)
1. [Pre-requisites](#pre-requisites)
1. [Step 1: Install the Sealed Secrets](#step-1-install-sealed-secrets)
1. [Step 2: Encrypt local secrets into Sealed Secrets](#step-2-encrypt-local-secrets-into-sealed-secrets)

## Introduction

[Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets) is commonly used for achieving declarative Kubernetes Secret Management. The project offers a mechanism to encrypt secrets locally. Since the Sealed Secrets are encrypted, they can be safely stored in a code repository. This enables an easy to implement GitOps flow that is very popular among the OSS community.

This guide walks you through the process of deploying Sealed Secrets in your cluster and installing an example secret.

## Pre-requisites

- Sealed Secrets assumes a working Kubernetes cluster (v1.16+), as well as the [`helm`](https://helm.sh/docs/intro/install/) (v3.1.0+) and [`kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl/) command-line interfaces installed and configured to talk to your Kubernetes cluster.

- Sealed Secrets has been tested with Amazon Elastic Kubernetes Service (EKS) Azure Kubernetes Service (AKS), Google Kubernetes Engine (GKE), minikube and Openshift.

## Step 1: Install Sealed Secrets

Sealed Secrets is composed of two parts:

- A cluster-side controller
- A client-side utility: kubeseal

### Installing the sealed-secrets-controller

The controller can be deployed using three different methods: direct yaml manifest installation, helm chart or carvel package.

#### Sealed Secrets manifest

Sealed secrets controller manifests are available from the [releases page](https://github.com/bitnami-labs/sealed-secrets/releases). You can choose the most convenient deployment for your cluster:

- `controller.yaml` Is a full manifest description of all the components required for the Sealed Secrets controller to operate. This includes Cluster role permissions and CRD definitions.
- `controller-norbac.yaml` Is a restricted version of the manifest descriptor. This version does not include CRDs nor Cluster roles.

#### Helm chart

The Sealed Secrets [Helm chart](https://helm.sh/) is officially supported and hosted in this GitHub repository.
```shell
helm repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
helm install sealed-secrets-controller sealed-secrets/sealed-secrets \
--set namespace=kube-system \
```

> The kubeseal CLI assumes that the controller is installed within the `kube-system` namespace by default with a deployment named `sealed-secrets-controller`. The above installation defines the same configuration to avoid unnecessary friction while using kubeseal.

#### Carvel package

It is also possible to install Sealed Secrets as a [Carvel package](https://carvel.dev/kapp-controller/docs/v0.46.0/packaging/). To do so, you'll need to install `kapp-controller` in the target cluster and then deploy the needed `Package` and `PackageInstall` manifests.

```shell
# Deploy kapp-controller
kapp deploy -a kc -f https://github.com/vmware-tanzu/carvel-kapp-controller/releases/latest/download/release.yml
# Deploy the Sealed Secrets package in the cluster
kapp deploy -a sealed-secrets-carvel -f https://raw.githubusercontent.com/bitnami-labs/sealed-secrets/main/carvel/package.yaml
Changes

Namespace  Name                              Kind     Conds.  Age  Op      Op st.  Wait to    Rs  Ri
default    sealedsecrets.bitnami.com.2.10.0  Package  -       -    create  -       reconcile  -   -
...
Succeeded

kubectl get Package
NAME                               PACKAGEMETADATA NAME        VERSION   AGE
sealedsecrets.bitnami.com.2.10.0   sealedsecrets.bitnami.com   2.10.0    18s
```

Once the Package is available, it'll be necessary to execute the PackageInstall action, following the [carvel documentation](https://carvel.dev/kapp-controller/docs/v0.35.0/packaging-tutorial/#installing-a-package).

### Installing the kubeseal CLI

#### Homebrew

The `kubeseal` client is available on [homebrew](https://formulae.brew.sh/formula/kubeseal):

```bash
brew install kubeseal
```

#### MacPorts

The `kubeseal` client is also available on [MacPorts](https://ports.macports.org/port/kubeseal/summary):

```bash
port install kubeseal
```

#### Nixpkgs

The `kubeseal` client is also available on [Nixpkgs](https://search.nixos.org/packages?channel=unstable&show=kubeseal&from=0&size=50&sort=relevance&type=packages&query=kubeseal): (**DISCLAIMER**: Not maintained by sealed secrets).

```bash
nix-env -iA nixpkgs.kubeseal
```

#### Linux

The `kubeseal` client can be installed on Linux, using the below commands:

```bash
wget https://github.com/bitnami-labs/sealed-secrets/releases/download/<release-tag>/kubeseal-<version>-linux-amd64.tar.gz
tar -xvzf kubeseal-<version>-linux-amd64.tar.gz kubeseal
sudo install -m 755 kubeseal /usr/local/bin/kubeseal
```

where `release-tag` is the [version tag](https://github.com/bitnami-labs/sealed-secrets/tags) of the kubeseal release you want to use. For example: `v0.21.0`.

#### Installation from source

If you just want the latest client tool, it can be installed into
`$GOPATH/bin` with:

```bash
go install github.com/bitnami-labs/sealed-secrets/cmd/kubeseal@main
```

You can specify a release tag or a commit SHA instead of `main`.

The `go install` command will place the `kubeseal` binary at `$GOPATH/bin`:

```bash
$(go env GOPATH)/bin/kubeseal
```

## Step 2: Encrypt local secrets into Sealed Secrets

```bash
# Create a json/yaml-encoded Secret somehow:
# (note use of `--dry-run` - this is just a local file!)
echo -n bar | kubectl create secret generic mysecret --dry-run=client --from-file=foo=/dev/stdin -o json >mysecret.json

# This is the important bit:
kubeseal -f mysecret.json -w mysealedsecret.json

# At this point mysealedsecret.json is safe to upload to Github,
# post on Twitter, etc.

# Eventually:
kubectl create -f mysealedsecret.json

# Profit!
kubectl get secret mysecret
```

> The `SealedSecret` and `Secret` must have **the same namespace and
name**. This is a feature to prevent other users on the same cluster
from re-using your sealed secrets.
