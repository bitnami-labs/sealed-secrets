# Kubeapps Pinniped-Proxy Developer Guide

`pinniped-proxy` proxies incoming requests with an `Authorization: Bearer token` header, exchanging the token via the pinniped aggregate API for x509 short-lived client certificates, before forwarding the request onwards to the destination k8s API server.

`pinniped-proxy` can be used by our Kubeapps frontend to ensure OIDC requests for the Kubernetes API server are forwarded through only after exchanging the OIDC id token for client certificates used by the Kubernetes API server, for situations where the Kubernetes API server is not configured for OIDC.

You can read more in the [investigation and POC design document for `pinniped-proxy`](https://docs.google.com/document/d/1WzDWQh1CDZ6fRg9Md-2l2l7JqVzFkZGACZA1WWog9AU/).

## Prerequisites

- [Git](https://git-scm.com/)
- [Rust programming language](https://www.rust-lang.org/tools/install)
- (more to come)

## Running in development

[`cargo`](https://doc.rust-lang.org/cargo/) is the Rust package manager tool is used for most development activities. Assuming you are already in the `cmd/pinniped-proxy` directory, you can always compile and run the executable with:

```bash
cargo run
```

and pass command-line options to the executable after a double-dash, for example:

```bash
cargo run -- -h
    Finished dev [unoptimized + debuginfo] target(s) in 0.05s
     Running `target/debug/pinniped-proxy -h`
pinniped-proxy 0.1.0
A proxy server which converts k8s API server requests with bearer tokens to requests with short-lived X509 certs
exchanged by pinniped.

pinniped-proxy proxies incoming requests with an `Authorization: Bearer token` header, exchanging the token via the
pinniped aggregate API for x509 short-lived client certificates, before forwarding the request onwards to the
destination k8s API server.

USAGE:
    pinniped-proxy [OPTIONS]

FLAGS:
    -h, --help       Prints help information
    -V, --version    Prints version information

OPTIONS:
    -p, --port <port>    Specify the port on which pinniped-proxy listens. [default: 3333]
```

## Running tests

Similarly, tests can be run with the cargo tool:

```bash
cargo test
```

## Running the local playground environment

### Prerequisite: getting the node IP

First of all, run `docker network inspect kind | jq '.[0].IPAM.Config[0].Gateway'` (adapt this line as you require, you only will need the Kind Gateway IP), get the IP and use the next one. For example, if you get `172.18.0.1` in your command, you will need `172.18.0.2`.

Next, follow the steps in [/docs/reference/developer/using-makefiles.md](../developer/using-makefiles.md) to modify the proper yaml files by replacing with this value.

Next, replace `172.18.0.2` with the previous IP (and `172.18.0.3` with the next one) the following files: - [script/makefiles/deploy-dev.mk](https://github.com/vmware-tanzu/kubeapps/blob/main/script/makefiles/deploy-dev.mk) - [kubeapps-local-dev-additional-kind-cluster-for-pinniped.yaml](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/reference/manifests/kubeapps-local-dev-additional-kind-cluster-for-pinniped.yaml) - [kubeapps-local-dev-auth-proxy-values.yaml](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/reference/manifests/kubeapps-local-dev-auth-proxy-values.yaml) - [kubeapps-local-dev-dex-values.yaml](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/reference/manifests/kubeapps-local-dev-dex-values.yaml)

### Launching the dev environment

- Run `make cluster-kind-for-pinniped` or `make multi-cluster-kind-for-pinniped` for multi-cluster.
- Update [https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/reference/manifests/kubeapps-local-dev-additional-kind-cluster-for-pinniped.yaml](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/reference/manifests/kubeapps-local-dev-additional-kind-cluster-for-pinniped.yaml) copying the certificate-authority-data from the additional cluster (`~/.kube/kind-config-kubeapps-additional`) to the `certificate-authority-data` of the second cluster.
- Run `make deploy-dev-for-pinniped` and additionally `make deploy-pinniped-additional` for multi-cluster.
- Edit [https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/reference/manifests/kubeapps-pinniped-jwt-authenticator.yaml](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/reference/manifests/kubeapps-pinniped-jwt-authenticator.yaml) with the `certificate-authority-data` from the main cluster (`~/.kube/kind-config-kubeapps`).
- Run `make add-pinniped-jwt-authenticator` and additionally `make add-pinniped-jwt-authenticator-additional` for multi-cluster.
- Open <https://localhost/> and login with `kubeapps-operator@example.com`/`password`

> Note: make sure you are really copying `certificate-authority-data` and not the `client-certificate-data` or `client-certificate-data`. Otherwise, the setup will not work.

### Update the pinniped-proxy image in your cluster

In order to test your local changes in your cluster, assuming you are using [Kind](https://kind.sigs.k8s.io), you will need to make the local container image, upload it to the cluster and patch the kubeapps deployment to use this new image:

```bash
IMAGE_TAG=dev make kubeapps/pinniped-proxy
kind load docker-image kubeapps/pinniped-proxy:dev --name kubeapps
kubectl set image -n kubeapps deployment kubeapps pinniped-proxy=kubeapps/pinniped-proxy:dev
kubectl delete pod -n kubeapps -l app=kubeapps
```

### View logs

Check the pinniped-proxy logs:

```bash
kubectl logs deployment/kubeapps pinniped-proxy -n kubeapps -f
```

Check the Pinnped logs:

```bash
kubectl logs deployment/pinniped-concierge -n pinniped-concierge -f
```

### Troubleshooting

Please have a look at the following guides:

- [Using an OIDC provider with Pinniped](../../howto/OIDC/using-an-OIDC-provider-with-pinniped.md).
- [Debugging auth failures when using OIDC](../../howto/OIDC/OAuth2OIDC-debugging.md).

Also, please verify that you have modified the Kind node IP accordingly and the `certificate-authority-data` has been properly copied to the corresponding files.

Find below some typical problems with a possible workaround:

- Missing permissions to access any namespace:
  - Add RBAC config by running `kubectl apply -f .https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/reference/manifests/kubeapps-local-dev-users-rbac.yaml`
- Not logging in even if everything is correct:
  - Delete and create the jwt authenticator: `make delete-pinniped-jwt-authenticator add-pinniped-jwt-authenticator`
  - If still not working, remove Pinniped and create it again: `make delete-pinniped make deploy-pinniped deploy-pinniped-additional`.
- When running the makefile it says `make: Nothing to be done for ...`:
  - Simply try using the `-B` flag in your make command to force it and see which error is really being thrown.

## Formatting the code

Using `rustfmt` formatting tool with cargo:

```bash
cargo fmt
```
