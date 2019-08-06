# "Sealed Secrets" for Kubernetes

[![](https://img.shields.io/github/release/bitnami-labs/sealed-secrets.svg)](https://github.com/bitnami-labs/sealed-secrets/releases/latest)
[![Build Status](https://travis-ci.org/bitnami-labs/sealed-secrets.svg?branch=master)](https://travis-ci.org/bitnami-labs/sealed-secrets)
[![Go Report Card](https://goreportcard.com/badge/github.com/bitnami-labs/sealed-secrets)](https://goreportcard.com/report/github.com/bitnami-labs/sealed-secrets)

**Problem:** "I can manage all my K8s config in git, except Secrets."

**Solution:** Encrypt your Secret into a SealedSecret, which *is* safe
to store - even to a public repository.  The SealedSecret can be
decrypted only by the controller running in the target cluster and
nobody else (not even the original author) is able to obtain the
original Secret from the SealedSecret.

## Installation

See https://github.com/bitnami-labs/sealed-secrets/releases for the latest
release.

```sh
$ release=$(curl --silent "https://api.github.com/repos/bitnami-labs/sealed-secrets/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')

# Install client-side tool into /usr/local/bin/
$ GOOS=$(go env GOOS)
$ GOARCH=$(go env GOARCH)
$ wget https://github.com/bitnami-labs/sealed-secrets/releases/download/$release/kubeseal-$GOOS-$GOARCH
$ sudo install -m 755 kubeseal-$GOOS-$GOARCH /usr/local/bin/kubeseal

# Note:  If installing on a GKE cluster, a ClusterRoleBinding may be needed to successfully deploy the controller in the final command.  Replace <your-email> with a valid email, and then deploy the cluster role binding:
$ USER_EMAIL=<your-email>
$ kubectl create clusterrolebinding $USER-cluster-admin-binding --clusterrole=cluster-admin --user=$USER_EMAIL

# Install SealedSecret CRD, server-side controller into kube-system namespace (by default)
# Note the second sealedsecret-crd.yaml file is not necessary for releases >= 0.8.0
$ kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/$release/controller.yaml
$ kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/$release/sealedsecret-crd.yaml
```

`controller.yaml` will create the `SealedSecret` resource and install the controller
into `kube-system` namespace, create a service account and necessary
RBAC roles.

After a few moments, the controller will start, generate a key pair,
and be ready for operation.  If it does not, check the controller
logs.

The key certificate (public key portion) is used for sealing secrets,
and needs to be available wherever `kubeseal` is going to be
used. The certificate is not secret information, although you need to
ensure you are using the correct file.

`kubeseal` will fetch the certificate from the controller at runtime
(requires secure access to the Kubernetes API server), which is
convenient for interactive use.  The recommended automation workflow
is to store the certificate to local disk with
`kubeseal --fetch-cert >mycert.pem`,
and use it offline with `kubeseal --cert mycert.pem`.
The certificate is also printed to the controller log on startup.

### Installation from source

If you just want the latest client tool, it can be installed into
`$GOPATH/bin` with:

```sh
% (cd /; GO111MODULE=on go get github.com/bitnami-labs/sealed-secrets/cmd/kubeseal@master)
go: finding github.com/bitnami-labs/sealed-secrets/cmd/kubeseal master
go: finding github.com/bitnami-labs/sealed-secrets/cmd master
go: finding github.com/bitnami-labs/sealed-secrets master
go: extracting github.com/bitnami-labs/sealed-secrets v0.8.1-0.20190724082116-385d02a4f4a3
```

You can specify a release tag or a commit SHA instead of `master`.

For a more complete development environment, clone the repository and
use the Makefile:

```sh
% git clone https://github.com/bitnami-labs/sealed-secrets.git
% cd sealed-secrets

# Build client-side tool and controller binaries
% make
```

## Usage

```sh
# Create a json/yaml-encoded Secret somehow:
# (note use of `--dry-run` - this is just a local file!)
$ kubectl create secret generic mysecret --dry-run --from-literal=foo=bar -o json >mysecret.json

# This is the important bit:
$ kubeseal <mysecret.json >mysealedsecret.json

# mysealedsecret.json is safe to upload to github, post to twitter,
# etc.  Eventually:
$ kubectl create -f mysealedsecret.json

# Profit!
$ kubectl get secret mysecret
```

Note the `SealedSecret` and `Secret` must have *the same namespace and
name*.  This is a feature to prevent other users on the same cluster
from re-using your sealed secrets.  `kubeseal` reads the namespace
from the input secret, accepts an explicit `--namespace` arg, and uses
the `kubectl` default namespace (in that order). Any labels,
annotations, etc on the original `Secret` are preserved, but not
automatically reflected in the `SealedSecret`.

By design, this scheme *does not authenticate the user*.  In other
words, *anyone* can create a `SealedSecret` containing any `Secret`
they like (provided the namespace/name matches).  It is up to your
existing config management workflow, cluster RBAC rules, etc to ensure
that only the intended `SealedSecret` is uploaded to the cluster.  The
only change from existing Kubernetes is that the *contents* of the
`Secret` are now hidden while outside the cluster.

## Details

This controller adds a new `SealedSecret` custom resource. The
interesting part of a `SealedSecret` is a base64-encoded
asymmetrically encrypted `Secret`.

The controller maintains a set of private/public key pairs as kubernetes
secrets. Keys are labelled with `sealedsecrets.bitnami.com/sealed-secrets-key`
and identified in the label as either `active` or `compromised`. On startup,
The sealed secrets controller will...
1. Search for these keys and add them to its local store if they are
labelled as active.
2. Create a new key
3. Start the key rotation cycle

#### Key rotation

Keys are automatically rotated. This can be configured on controller startup with
the `--rotate-period=<value>` flag. The `value` field can be given as golang
duration flag (eg: `720h30m`).

A key can be generated early in two ways
1. Send `SIGUSR1` to the controller
`kubectl exec -it <controller pod> -- kill -SIGUSR1 1`
2. Label the current latest key as compromised (any value other than active)
`kubectl label secrets <keyname> sealedsecrets.bitnami.com/sealed-secrets-key=compromised`.

**NOTE** Sealed secrets currently does not automatically pick up relabelled
keys, an admin must restart the controller before the effect will apply.

Labelling a secret with anything other than `active` effectively deletes
the key from the sealed secrets controller, but it is still available in k8s for
manual encryption/decryption if need be.

## Developing
To be able to develop on this project, you need to have the following tools installed:
* make
* [Ginkgo](https://onsi.github.io/ginkgo/)
* [Minikube](https://github.com/kubernetes/minikube)
* [kubecfg](https://github.com/bitnami/kubecfg)
* Go

To build the `kubeseal` and controller binaries, run:
```bash
$ make
```

To run the unit tests:
```bash
$ make test
```

To run the integration tests:
* Start Minikube
* Build the controller for Linux, so that it can be run within a Docker image - edit the Makefile to add
`GOOS=linux GOARCH=amd64` to `%-static`, and then run `make controller.yaml IMAGE_PULL_POLICY=Never`
* Add the sealed-secret CRD and controller to Kubernetes - `kubectl apply -f controller.yaml`
* Revert any changes made to the Makefile to build the Linux controller
* Remove the binaries which were possibly built for another OS - `make clean`
* Rebuild the binaries for your OS - `make`
* Run the integration tests - `make integrationtest`

To update the jsonnet dependencies:

```
$ jb install --jsonnetpkg-home=jsonnet_vendor
```

## Helm Chart
Sealed Secret helm charts can be found on this [link](https://github.com/helm/charts/tree/master/stable/sealed-secrets)

## FAQ

- Will you still be able to decrypt if you no longer have access to your cluster?

No, the private key is only stored in the Secret managed by the controller (unless you have some other backup of your k8s objects). There are no backdoors - without that private key, then you can't decrypt the SealedSecrets. If you can't get to the Secret with the encryption key, and you also can't get to the decrypted versions of your Secrets live in the cluster, then you will need to regenerate new passwords for everything, seal them again with a new sealing key, etc.

- How can I do a backup of my SealedSecrets?

If you do want to make a backup of the encryption private key, it's easy to do from an account with suitable access and:

```
$ kubectl get secret -n kube-system sealed-secrets-key -o yaml >master.key
```

NOTE: This is the controller's public + private key and should be kept omg-safe!

To restore from a backup after some disaster, just put that secret back before starting the controller - or if the controller was already started, replace the newly-created secret and restart the controller:

```
$ kubectl replace -f master.key
$ kubectl delete pod -n kube-system -l name=sealed-secrets-controller
```

- What flags are available for kubeseal?

You can check the flags available using `kubeseal --help`.

## Community

- [#sealed-secrets on Kubernetes Slack](https://kubernetes.slack.com/messages/sealed-secrets)

Click [here](http://slack.k8s.io) to sign up to the Kubernetes Slack org.
