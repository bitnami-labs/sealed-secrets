# "Sealed Secrets" for Kubernetes

[![](https://img.shields.io/badge/install-docs-brightgreen.svg)](#Installation)
[![](https://img.shields.io/github/release/bitnami-labs/sealed-secrets.svg)](https://github.com/bitnami-labs/sealed-secrets/releases/latest)
[![](https://img.shields.io/homebrew/v/kubeseal)](https://formulae.brew.sh/formula/kubeseal)
[![Build Status](https://travis-ci.org/bitnami-labs/sealed-secrets.svg?branch=master)](https://travis-ci.org/bitnami-labs/sealed-secrets)
[![Go Report Card](https://goreportcard.com/badge/github.com/bitnami-labs/sealed-secrets)](https://goreportcard.com/report/github.com/bitnami-labs/sealed-secrets)
![Downloads](https://img.shields.io/github/downloads/bitnami-labs/sealed-secrets/total.svg)

**Problem:** "I can manage all my K8s config in git, except Secrets."

**Solution:** Encrypt your Secret into a SealedSecret, which *is* safe
to store - even to a public repository.  The SealedSecret can be
decrypted only by the controller running in the target cluster and
nobody else (not even the original author) is able to obtain the
original Secret from the SealedSecret.

## Overview

Sealed Secrets is composed of two parts:

* A cluster-side controller / operator
* A client-side utility: `kubeseal`

The `kubeseal` utility uses asymmetric crypto to encrypt secrets that only the controller can decrypt.

These encrypted secrets are encoded in a `SealedSecret` resource, which you can see as a recipe for creating
a secret. Here is how it looks:

```yaml
apiVersion: bitnami.com/v1alpha1
kind: SealedSecret
metadata:
  name: mysecret
  namespace: mynamespace
spec:
  encryptedData:
    foo: AgBy3i4OJSWK+PiTySYZZA9rO43cGDEq.....
```

Once unsealed this will produce a secret equivalent to this:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mysecret
  namespace: mynamespace
data:
  foo: bar  # <- base64 encoded "bar"
```

This normal [kubernetes secret](https://kubernetes.io/docs/concepts/configuration/secret/) will appear in the cluster
after a few seconds and you can use it as you would use any secret that you would have created directly (e.g. reference it from a `Pod`).

Jump to the [Installation](#installation) section to get up and running.

The [Usage](#usage) section explores in more detail how you craft `SealedSecret` resources.

### SealedSecrets as templates for secrets

The previous example only focused on the encrypted secret items themselves, but the relationship between a `SealedSecret` custom resource and the `Secret` it unseals into is similar in many ways (but not in all of them) to the familiar `Deployment` vs `Pod`.

In particular, the annotations and labels of a `SealedSecret` resource are not the same as the annotations of the `Secret` that gets generated out of it.

To capture this distinction, the `SealedSecret` object has a `template` section which encodes all the fields you want the controller to put in the unsealed `Secret`.

This includes metadata such as labels or annotations, but also things like the `type` of the secret.

```yaml
apiVersion: bitnami.com/v1alpha1
kind: SealedSecret
metadata:
  name: mysecret
  namespace: mynamespace
  annotation:
    "kubectl.kubernetes.io/last-applied-configuration": ....
spec:
  encryptedData:
    .dockercfg: AgBy3i4OJSWK+PiTySYZZA9rO43cGDEq.....
  template:
    type: kubernetes.io/dockercfg
    # this is an example of labels and annotations that will be added to the output secret
    metadata:
      labels:
        "jenkins.io/credentials-type": usernamePassword
      annotations:
        "jenkins.io/credentials-description": credentials from Kubernetes
```

The controller would unseal that into something like:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mysecret
  namespace: mynamespace
  labels:
    "jenkins.io/credentials-type": usernamePassword
  annotations:
    "jenkins.io/credentials-description": credentials from Kubernetes
  ownerReferences:
  - apiVersion: bitnami.com/v1alpha1
    controller: true
    kind: SealedSecret
    name: mysecret
    uid: 5caff6a0-c9ac-11e9-881e-42010aac003e
type: kubernetes.io/dockercfg
data:
  .dockercfg: ewogICJjcmVk...
```

As you can see, the generated `Secret` resource is a "dependent object" of the `SealedSecret` and as such
it will be updated and deleted whenever the `SealedSecret` object gets updated or deleted.


### Public key / Certificate

The key certificate (public key portion) is used for sealing secrets,
and needs to be available wherever `kubeseal` is going to be
used. The certificate is not secret information, although you need to
ensure you are using the correct one.

`kubeseal` will fetch the certificate from the controller at runtime
(requires secure access to the Kubernetes API server), which is
convenient for interactive use, but it's known to be brittle when users
have clusters with special configurations such as [private GKE clusters](docs/GKE.md#private-gke-clusters) that have
firewalls between master and nodes.

An alternative workflow
is to store the certificate somewhere (e.g. local disk) with
`kubeseal --fetch-cert >mycert.pem`,
and use it offline with `kubeseal --cert mycert.pem`.
The certificate is also printed to the controller log on startup.


> **NOTE**: we are working on providing key management mechanisms that offload the encryption to HSM based modules or managed cloud crypto solutions such as KMS.

## Installation

See https://github.com/bitnami-labs/sealed-secrets/releases for the latest
release and detailed installation instructions.

Cloud platform specific notes and instructions:
* [GKE](docs/GKE.md)

### Controller

Once you deploy the manifest it will create the `SealedSecret` resource
and install the controller into `kube-system` namespace, create a service
account and necessary RBAC roles.

After a few moments, the controller will start, generate a key pair,
and be ready for operation.  If it does not, check the controller
logs.

### Kustomize

The official controller manifest installation mechanism is just a YAML file.

In some cases you might need to apply your own customizations, like set a custom namespace or set some env variables.

`kubectl` has native support for that, see [kustomize](https://kustomize.io/).

### Helm Chart

Sealed Secret helm charts can be found on this [link](https://github.com/helm/charts/tree/master/stable/sealed-secrets). It's maintained independently and it might lag a bit behind the latest release.

### Homebrew

The `kubeseal` client is also available on [homebrew](https://formulae.brew.sh/formula/kubeseal):

```
$ brew install kubeseal
```

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

## Upgrade

Don't forget to check the [release notes](RELEASE-NOTES.md) for guidance about
possible breaking changes when you upgrade the client tool
and/or the controller.

## Usage

```sh
# Create a json/yaml-encoded Secret somehow:
# (note use of `--dry-run` - this is just a local file!)
$ echo -n bar | kubectl create secret generic mysecret --dry-run --from-literal=foo=/dev/stdin -o json >mysecret.json

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

## Secret Rotation

You should always rotate your secrets. But since your secrets are encrypted with another secret,
you need to understand how these two layers relate in order to take the right decisions.

TL;DR:

> If a *sealing* private key is compromised, you need to follow the instructions below in "Early key renewal"
> section before rotating any of your actual secret values.
>
> SealedSecret key renewal and re-encryption features are **not a substitute** for periodical rotation of your actual secret values.

### Key renewal

Keys are automatically renewed every 30 days. This can be configured on controller startup with
the `--rotate-period=<value>` flag. The `value` field can be given as golang
duration flag (eg: `720h30m`). A value of `0` will disable automatic key renewal.

> Unfortunately you cannot use e.g. "d" as a unit for days because that's not supported by the Go stdlib. Instead of hitting your face with a palm, take this as an opportunity to meditate on the [falsehoods programmers believe about time](https://infiniteundo.com/post/25326999628/falsehoods-programmers-believe-about-time).

The feature has been historically called "key rotation" but this term can be confusing.
Sealed secrets are not automatically rotated and old keys are not deleted
when new keys are generated. Old sealed secrets resources can be still decrypted.

### User secret rotation

The *sealing key* renewal and SealedSecret rotation are **not a substitute** for rotating your actual secrets.

A core value proposition of this tool is:

> Encrypt your Secret into a SealedSecret, which *is* safe to store - even to a public repository.

If you store anything in a version control storage, and in a public one in particular, you must assume
you cannot ever delete that information.

*If* a sealing key somehow leaks out of the cluster you must consider all your SealedSecret resources
encrypted with that key as compromised. No amount of sealing key rotation in the cluster or even re-encryption of existing SealedSecrets files can change that.

The best practice is to periodically rotate all your actual secrets (e.g. change the password) **and** craft new
SealedSecret resource with those new secrets.

But if the sealed secrets controller were not renewing the *sealing key* that rotation would be moot,
since the attacker could just decrypt the new secrets as well. Thus you need to do both: periodically renew the sealing key and rotate your actual secrets!

### Early key renewal

If you know or suspect a *sealing key* has been compromised you should renew the key ASAP before you
start sealing your new rotated secrets, otherwise you'll be giving attackers access to your new secrets as well.

A key can be generated early in two ways
1. Label the current latest key as compromised (any value other than active)
`kubectl label secrets <keyname> sealedsecrets.bitnami.com/sealed-secrets-key=compromised`.
2. Send `SIGUSR1` to the controller
`kubectl exec -it <controller pod> -- kill -SIGUSR1 1`

**NOTE** Sealed secrets currently does not automatically pick up relabelled
keys, an admin must restart the controller before the effect will apply.

Labelling a secret with anything other than `active` effectively deletes
the key from the sealed secrets controller, but it is still available in k8s for
manual encryption/decryption if need be.

### Re-encryption

Before you can get rid of some old sealing keys you need to re-encrypt your SealedSecrets with the latest private key).

```bash
kubeseal --rotate <my_sealed_secret.json >tmp.json \
  && mv tmp.json my_sealed_secret.json
```

The invocation above will produce a new sealed secret file freshly encrypted with
the latest key, without making the secrets leave the cluster to the client. You can then save that file
in your version control system (`kubeseal --rotate` doesn't update the in-cluster object).

Currently old keys are not garbage collected automatically.

It's a good idea to periodically re-encrypt your SealedSecrets. But as mentioned above, don't lull yourself in a false sense of security: you must assume the old version of the SealedSecret (the one encrypted with a key you think of as dead) is still potentially around and accessible to attackers. I.e. re-encryption is not a substitute for periodically rotating your actual secrets.

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
