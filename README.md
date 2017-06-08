# "Sealed Secrets" for Kubernetes

**Problem:** "I can manage all my K8s config in git, except Secrets."

**Solution:** Encrypt your Secret into a SealedSecret, which *is* safe
to store - even to a public repository.  The SealedSecret can be
decrypted only by the controller running in the target cluster and
nobody else (not even the original author) is able to obtain the
original Secret from the SealedSecret.

## Installation

See https://github.com/ksonnet/sealed-secrets/releases for the latest
release.

```sh
# Install client-side tool into $GOPATH/bin
$ go get github.com/ksonnet/sealed-secrets/cmd/ksonnet-seal

# Install server-side controller into kube-system namespace (by default)
$ kubectl create -f https://github.com/ksonnet/sealed-secrets/releases/download/v0.0.1b/controller.yaml
```

`controller.yaml` will create the `SealedSecret` third-party-resource,
install the controller into `kube-system` namespace, create a service
account and necessary RBAC roles.

After a few moments, the controller will start, generate a key pair,
and be ready for operation.  If it does not, check the controller
logs.

The key certificate (public key portion) is used for sealing secrets,
and needs to be installed wherever `ksonnet-seal` is going to be
used. The certificate is not secret information, although you need to
ensure you are using the correct file.

The certificate is printed to the controller log on startup, and can
also be retrieved directly from the underlying secret (the latter
requires access to the sealing secret, which is generally an
undesirable thing). (TODO: Improve this part)

```sh
# Fetch cluster-wide certificate used for encrypting.
# The certificate is also printed to the controller log on startup.
$ kubectl get secret -n kube-system sealed-secrets-key -o go-template='{{index .data "tls.crt"}}' | base64 -d > seal.crt
```

## Usage

```sh
# This is the important bit:
$ ksonnet-seal --cert seal.crt <mysecret.json >mysealedsecret.json

# mysealedsecret.json is safe to upload to github, post to twitter,
# etc.  Eventually:
$ kubectl create -f mysealedsecret.json

# Profit!
$ kubectl get secret mysecret
```

Note the `SealedSecret` and `Secret` must have *the same namespace and
name*.  This is a feature to prevent other users on the same cluster
from re-using your sealed secrets.  Any labels, annotations, etc on
the original `Secret` are preserved, but not automatically reflected
in the `SealedSecret`.

## Details

This controller adds a new `SealedSecret` third-party resource.  The
interesting part of which is a base64-encoded asymmetrically encrypted
`Secret`.

The controller looks for a cluster-wide private/public key pair on
startup, and generates a new key pair if not found.  The key is
persisted in a regular `Secret` in the same namespace as the
controller.  The public key portion of this (in the form of a
self-signed certificate) should be made publicly available to anyone
wanting to use `SealedSecret`s with this cluster.

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
