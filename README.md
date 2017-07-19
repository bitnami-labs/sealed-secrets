# "Sealed Secrets" for Kubernetes

[![Build Status](https://travis-ci.org/bitnami/sealed-secrets.svg?branch=master)](https://travis-ci.org/bitnami/sealed-secrets)
[![Go Report Card](https://goreportcard.com/badge/github.com/bitnami/sealed-secrets)](https://goreportcard.com/report/github.com/bitnami/sealed-secrets)

**Problem:** "I can manage all my K8s config in git, except Secrets."

**Solution:** Encrypt your Secret into a SealedSecret, which *is* safe
to store - even to a public repository.  The SealedSecret can be
decrypted only by the controller running in the target cluster and
nobody else (not even the original author) is able to obtain the
original Secret from the SealedSecret.

## Installation

See https://github.com/bitnami/sealed-secrets/releases for the latest
release.

```sh
$ release=$(curl --silent "https://api.github.com/repos/bitnami/sealed-secrets/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')

# Install client-side tool into /usr/local/bin/
$ GOOS=$(go env GOOS)
$ GOARCH=$(go env GOARCH)
$ wget https://github.com/bitnami/sealed-secrets/releases/download/$release/kubeseal-$GOOS-$GOARCH
$ sudo install -m 755 kubeseal-$GOOS-$GOARCH /usr/local/bin/kubeseal

# Install server-side controller into kube-system namespace (by default)
$ kubectl create -f https://github.com/bitnami/sealed-secrets/releases/download/$release/controller.yaml
```

`controller.yaml` will create the `SealedSecret` third-party-resource,
install the controller into `kube-system` namespace, create a service
account and necessary RBAC roles.

After a few moments, the controller will start, generate a key pair,
and be ready for operation.  If it does not, check the controller
logs.

The key certificate (public key portion) is used for sealing secrets,
and needs to be available wherever `kubeseal` is going to be
used. The certificate is not secret information, although you need to
ensure you are using the correct file.

`kubeseal` will fetch the certificate from the controller at
runtime (requires secure access to the Kubernetes API server), but can
also be read from a local file for offline situations (eg: automated
jobs).  The certificate is also printed to the controller log on
startup.

### Installation from source

If you just want the latest client tool, it can be installed into
`$GOPATH/bin` with:

```sh
% go get github.com/bitnami/sealed-secrets/cmd/kubeseal
```

For a more complete development environment, clone the repository and
use the Makefile:

```sh
% git clone https://github.com/bitnami/sealed-secrets.git
% cd sealed-secrets

# Build client-side tool and controller binaries
% make
```

## Usage

```sh
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

This controller adds a new `SealedSecret` third-party resource.  The
interesting part of which is a base64-encoded asymmetrically encrypted
`Secret`.

The controller looks for a cluster-wide private/public key pair on
startup, and generates a new key pair if not found.  The key is
persisted in a regular `Secret` in the same namespace as the
controller.  The public key portion of this (in the form of a
self-signed certificate) should be made publicly available to anyone
wanting to use `SealedSecret`s with this cluster.  The certificate is
printed to the controller log at startup, and available via an HTTP
GET to `/v1/cert.pem` on the controller.

During encryption, the original `Secret` is JSON-encoded and
symmetrically encrypted using AES-GCM with a randomly-generated
single-use session key.  The session key is then asymmetrically
encrypted with the controller's public key using RSA-OAEP, and the
original `Secret`'s namespace/name as the OAEP input parameter (aka
label).  The final output is: 2 byte encrypted session key length ||
encrypted session key || encrypted Secret.

Note that during decryption by the controller, the `SealedSecret`'s
namespace/name is used as the OAEP input parameter, ensuring that the
`SealedSecret` and `Secret` are tied to the same namespace and name.

The generated `Secret` is marked as "owned" by the `SealedSecret` and
will be garbage collected if the `SealedSecret` is deleted.

## FAQ

- Will you still be able to decrypt if you no longer have access to your cluster?

No, the private key is only stored in the Secret managed by the controller (unless you have some other backup of your k8s objects). There are no backdoors - without that private key, then you can't decrypt the SealedSecrets. If you can't get to the Secret with the encryption key, and you also can't get to the decrypted versions of your Secrets live in the cluster, then you will need to regenerate new passwords for everything, seal them again with a new sealing key, etc.

- How can I do a backup of my SealedSecrets? 

If you do want to make a backup of the encryption private key, it's easy to do from an account with suitable access and:

`kubectl get secret -n kube-system sealed-secrets-key -o yaml >master.key`

NOTE: This is the controller's public + private key and should be kept omg-safe!

To restore from a backup after some disaster, just put that secret back before starting the controller - or if the controller was already started, replace the newly-created secret and restart the controller:

`kubectl replace secret -n kube-system sealed-secrets-key master.key`
`kubectl delete pod -n kube-system -l name=sealed-secrets-controller`
