# "Sealed Secrets" for Kubernetes

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
% go get github.com/bitnami-labs/sealed-secrets/cmd/kubeseal
```

For a more complete development environment, clone the repository and
use the Makefile:

```sh
% git clone https://github.com/bitnami-labs/sealed-secrets.git
% cd sealed-secrets

# Build client-side tool and controller binaries
% make
```

## Usage

**WARNING**: A bug in the current version is limiting secrets to use the "opaque" type. If you need to use another secret type (eg: `kubernetes.io/dockerconfigjson`), please use kubeseal from release 0.5.1 until [#86](https://github.com/bitnami-labs/sealed-secrets/issues/86) and [#92](https://github.com/bitnami-labs/sealed-secrets/issues/92) are resolved.

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

This controller adds a new `SealedSecret` custom resource.  The
interesting part of a `SealedSecret` is a base64-encoded
asymmetrically encrypted `Secret`.

The controller maintains a list of private/public key pairs, each
identified by a name. A new pair is added periodically, with the names
stored to the list. On startup, the controller does a few things.

1. Look for an existing `keynamelist`.
2. If a list is found, the controller cycles through each name, and
looks for a secret with that name. The secret contains a private/public
key pair for that name, and is added to the controllers local key registry.
3. If a list is not found, the controller with create the list.
4. Regardless of whether a list is found or not, the controller will
generate a new private/public key pair and store it to both kubernetes
and the local store.

The private/public key pairs are, by default, 4096 bit RSA key pairs.
The pairs and the list of key names are persisted in regular `Secret`
objects in the same namespace as the controller. These keys are generated
on a 14 day (default) cycle, and can be generated early in the container
(more on this later). The public keys are stored in the form of a self-
signed certificate, and should be made available to anyone wanting to
use `SealedSecret`s with the cluster. Each certificate is printed to the
controller log when they are generated, and avaible via an HTTP GET to
`/v1/cert.pem?keyname=<name>`. The keyname may be ommitted to retrieve
the latest public key.

During encryption, each value in the original `Secret` is
symmetrically encrypted using AES-GCM (AES-256) with a randomly-generated
single-use 32 byte session key.  The session key is then asymmetrically
encrypted with the controller's public key using RSA-OAEP (using SHA256), and the
original `Secret`'s namespace/name as the OAEP input parameter (aka
label).  The final output is: 2 byte encrypted session key length ||
encrypted session key || encrypted Secret.

Note that during decryption by the controller, the `SealedSecret`'s
namespace/name is used as the OAEP input parameter, ensuring that the
`SealedSecret` and `Secret` are tied to the same namespace and name.

The generated `Secret` is marked as "owned" by the `SealedSecret` and
will be garbage collected if the `SealedSecret` is deleted.

After sime time, the key used to encrypt a `SealedSecret` is replaced by
a newer key. The `SealedSecret` can still be decrypted by the old key,
but it is recommended to update with the latest key. This can be a
problem without access to the private key outside the cluster.

The `SealedSecret` can be re-encrypted by the cluster using an HTTP POST
to `/v1/rotate`.

**WARNING**
A current issue in serializing the data prevents the `/v1/rotate` endpoint
from returning a valid `SealedSecret` object. A workaround exists in the
`./kubeseal --rotate` flag.

### Blacklisting
Sealed secrets provides a blacklisting feature to warn users of secrets
that may have been encrypted with a compromised private key. The
controller maintains a list of key names corresponding to keys that are
suspected to be compromised. This list is also stored as a secret in
the controllers namespace and maintained similarly to the key list
itself.

The `SealedSecret`s controller will refuse to work with blacklisted
private and public keys. This will cause operations involving the
blacklisted keys to fail, including

1. Validating a `SealedSecret`
2. Rotating a `SealedSecret` to use the latest key
3. Retrieving a public key

If a `SealedSecret` is encrypted with a compromised private key, it is
recommended to change the data that is stored in it, and then re-encrypt
it.

### Controller Admin

Sealed secrets provides in-container controlls for managing the key
generation and blacklisting. With access to the container, the following
commands are available

* `ssadmin --key-gen`. Force the controller to generate a new private/
public key pair immediately before the scheduled time.
* `ssadmin --blacklist=<name>`. Blacklist a key name. If the key happens
to be the latest key, the controller will automatically generate a new
pair.

These flags can be used together. In the case of using `--key-gen` and
`--blacklist=<latest_keyname>`, the controller will only generate one new
pair.

## Developing
To be able to develop on this project, you need to have the following tools installed:
* make
* [Ginkgo](https://onsi.github.io/ginkgo/)
* [Minikube](https://github.com/kubernetes/minikube)
* [kubecfg](https://github.com/ksonnet/kubecfg)
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
`GOOS=linux GOARCH=amd64` to `%-static`, and then run `make controller.yaml `
* Alter `controller.yaml` so that `imagePullPolicy: Never`, to ensure that the image you've just built will be
used by Kubernetes
* Add the sealed-secret CRD and controller to Kubernetes - `kubectl apply -f controller.yaml`
* Revert any changes made to the Makefile to build the Linux controller
* Remove the binaries which were possibly built for another OS - `make clean`
* Rebuild the binaries for your OS - `make`
* Run the integration tests - `make integrationtest`


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
