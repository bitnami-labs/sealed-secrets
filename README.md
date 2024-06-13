# "Sealed Secrets" for Kubernetes

[![](https://img.shields.io/badge/install-docs-brightgreen.svg)](#Installation)
[![](https://img.shields.io/github/release/bitnami-labs/sealed-secrets.svg)](https://github.com/bitnami-labs/sealed-secrets/releases/latest)
[![](https://img.shields.io/homebrew/v/kubeseal)](https://formulae.brew.sh/formula/kubeseal)
[![Build Status](https://github.com/bitnami-labs/sealed-secrets/actions/workflows/ci.yml/badge.svg)](https://github.com/bitnami-labs/sealed-secrets/actions/workflows/ci.yml)
[![](https://img.shields.io/github/v/release/bitnami-labs/sealed-secrets?include_prereleases&label=helm&sort=semver)](https://github.com/bitnami-labs/sealed-secrets/releases)
[![Download Status](https://img.shields.io/docker/pulls/bitnami/sealed-secrets-controller.svg)](https://hub.docker.com/r/bitnami/sealed-secrets-controller)
[![Go Report Card](https://goreportcard.com/badge/github.com/bitnami-labs/sealed-secrets)](https://goreportcard.com/report/github.com/bitnami-labs/sealed-secrets)
![Downloads](https://img.shields.io/github/downloads/bitnami-labs/sealed-secrets/total.svg)

**Problem:** "I can manage all my K8s config in git, except Secrets."

**Solution:** Encrypt your Secret into a SealedSecret, which *is* safe
to store - even inside a public repository. The SealedSecret can be
decrypted only by the controller running in the target cluster and
nobody else (not even the original author) is able to obtain the
original Secret from the SealedSecret.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Overview](#overview)
  - [SealedSecrets as templates for secrets](#sealedsecrets-as-templates-for-secrets)
  - [Public key / Certificate](#public-key--certificate)
  - [Scopes](#scopes)
- [Installation](#installation)
  - [Controller](#controller)
    - [Kustomize](#kustomize)
    - [Helm Chart](#helm-chart)
  - [Kubeseal](#kubeseal)
    - [Homebrew](#homebrew)
    - [MacPorts](#macports)
    - [Linux](#linux)
    - [Installation from source](#installation-from-source)
- [Upgrade](#upgrade)
- [Usage](#usage)
  - [Managing existing secrets](#managing-existing-secrets)
  - [Patching existing secrets](#patching-existing-secrets)
  - [Update existing secrets](#update-existing-secrets)
  - [Raw mode (experimental)](#raw-mode-experimental)
  - [Validate a Sealed Secret](#validate-a-sealed-secret)
- [Secret Rotation](#secret-rotation)
  - [Sealing key renewal](#sealing-key-renewal)
  - [User secret rotation](#user-secret-rotation)
  - [Early key renewal](#early-key-renewal)
  - [Common misconceptions about key renewal](#common-misconceptions-about-key-renewal)
  - [Manual key management (advanced)](#manual-key-management-advanced)
  - [Re-encryption (advanced)](#re-encryption-advanced)
- [Details (advanced)](#details-advanced)
  - [Crypto](#crypto)
- [Developing](#developing)
- [FAQ](#faq)
  - [Will you still be able to decrypt if you no longer have access to your cluster?](#will-you-still-be-able-to-decrypt-if-you-no-longer-have-access-to-your-cluster)
  - [How can I do a backup of my SealedSecrets?](#how-can-i-do-a-backup-of-my-sealedsecrets)
  - [Can I decrypt my secrets offline with a backup key?](#can-i-decrypt-my-secrets-offline-with-a-backup-key)
  - [What flags are available for kubeseal?](#what-flags-are-available-for-kubeseal)
  - [How do I update parts of JSON/YAML/TOML/.. file encrypted with sealed secrets?](#how-do-i-update-parts-of-jsonyamltoml-file-encrypted-with-sealed-secrets)
  - [Can I bring my own (pre-generated) certificates?](#can-i-bring-my-own-pre-generated-certificates)
  - [How to use kubeseal if the controller is not running within the `kube-system` namespace?](#how-to-use-kubeseal-if-the-controller-is-not-running-within-the-kube-system-namespace)
  - [How to verify the images?](#how-to-verify-the-images)
  - [How to use one controller for a subset of namespaces](#How-to-use-one-controller-for-a-subset-of-namespaces)

- [Community](#community)
  - [Related projects](#related-projects)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Overview

Sealed Secrets is composed of two parts:

- A cluster-side controller / operator
- A client-side utility: `kubeseal`

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
  foo: YmFy  # <- base64 encoded "bar"
```

This normal [kubernetes secret](https://kubernetes.io/docs/concepts/configuration/secret/) will appear in the cluster
after a few seconds you can use it as you would use any secret that you would have created directly (e.g. reference it from a `Pod`).

Jump to the [Installation](#installation) section to get up and running.

The [Usage](#usage) section explores in more detail how you craft `SealedSecret` resources.

### SealedSecrets as templates for secrets

The previous example only focused on the encrypted secret items themselves, but the relationship between a `SealedSecret` custom resource and the `Secret` it unseals into is similar in many ways (but not in all of them) to the familiar `Deployment` vs `Pod`.

In particular, the annotations and labels of a `SealedSecret` resource are not the same as the annotations of the `Secret` that gets generated out of it.

To capture this distinction, the `SealedSecret` object has a `template` section which encodes all the fields you want the controller to put in the unsealed `Secret`.

The [Sprig function library](https://masterminds.github.io/sprig/) is available in addition to the default Go Text Template functions.

The `metadata` block is copied as is (the `ownerReference` field will be updated [unless disabled](#seal-secret-which-can-skip-set-owner-references)).

Other secret fields are handled individually. The `type` and `immutable` fields are copied, and the `data` field can be used to [template complex values](docs/examples/config-template) on the `Secret`. All other fields are currently ignored.

```yaml
apiVersion: bitnami.com/v1alpha1
kind: SealedSecret
metadata:
  name: mysecret
  namespace: mynamespace
  annotations:
    "kubectl.kubernetes.io/last-applied-configuration": ....
spec:
  encryptedData:
    .dockerconfigjson: AgBy3i4OJSWK+PiTySYZZA9rO43cGDEq.....
  template:
    type: kubernetes.io/dockerconfigjson
    immutable: true
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
type: kubernetes.io/dockerconfigjson
immutable: true
data:
  .dockerconfigjson: ewogICJjcmVk...
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
firewalls between control plane and nodes.

An alternative workflow
is to store the certificate somewhere (e.g. local disk) with
`kubeseal --fetch-cert >mycert.pem`,
and use it offline with `kubeseal --cert mycert.pem`.
The certificate is also printed to the controller log on startup.

Since v0.9.x certificates get automatically renewed every 30 days. It's good practice that you and your team
update your offline certificate periodically. To help you with that, since v0.9.2 `kubeseal` accepts URLs too. You can set up your internal automation to publish certificates somewhere you trust.

```bash
kubeseal --cert https://your.intranet.company.com/sealed-secrets/your-cluster.cert
```

It also recognizes the `SEALED_SECRETS_CERT` env var. (pro-tip: see also [direnv](https://github.com/direnv/direnv)).

> **NOTE**: we are working on providing key management mechanisms that offload the encryption to HSM based modules or managed cloud crypto solutions such as KMS.

### Scopes

SealedSecrets are from the POV of an end user a "write only" device.

The idea is that the SealedSecret can be decrypted only by the controller running in the target cluster and
nobody else (not even the original author) is able to obtain the original Secret from the SealedSecret.

The user may or may not have direct access to the target cluster.
More specifically, the user might or might not have access to the Secret unsealed by the controller.

There are many ways to configure RBAC on k8s, but it's quite common to forbid low-privilege users
from reading Secrets. It's also common to give users one or more namespaces where they have higher privileges,
which would allow them to create and read secrets (and/or create deployments that can reference those secrets).

Encrypted `SealedSecret` resources are designed to be safe to be looked at without gaining any knowledge about the secrets it conceals. This implies that we cannot allow users to read a SealedSecret meant for a namespace they wouldn't have access to
and just push a copy of it in a namespace where they can read secrets from.

Sealed-secrets thus behaves *as if* each namespace had its own independent encryption key and thus once you
seal a secret for a namespace, it cannot be moved in another namespace and decrypted there.

We don't technically use an independent private key for each namespace, but instead we *include* the namespace name
during the encryption process, effectively achieving the same result.

Furthermore, namespaces are not the only level at which RBAC configurations can decide who can see which secret. In fact, it's possible that users can access a secret called `foo` in a given namespace but not any other secret in the same namespace. We cannot thus by default let users freely rename `SealedSecret` resources otherwise a malicious user would be able to decrypt any SealedSecret for that namespace by just renaming it to overwrite the one secret user does have access to. We use the same mechanism used to include the namespace in the encryption key to also include the secret name.

That said, there are many scenarios where you might not care about this level of protection. For example, the only people who have access to your clusters are either admins or they cannot read any `Secret` resource at all. You might have a use case for moving a sealed secret to other namespaces (e.g. you might not know the namespace name upfront), or you might not know the name of the secret (e.g. it could contain a unique suffix based on the hash of the contents etc).

These are the possible scopes:

- `strict` (default): the secret must be sealed with exactly the same *name* and *namespace*. These attributes become *part of the encrypted data* and thus changing name and/or namespace would lead to "decryption error".
- `namespace-wide`: you can freely *rename* the sealed secret within a given namespace.
- `cluster-wide`: the secret can be unsealed in *any* namespace and can be given *any* name.

In contrast to the restrictions of *name* and *namespace*, secret *items* (i.e. JSON object keys like `spec.encryptedData.my-key`) can be renamed at will without losing the ability to decrypt the sealed secret.

The scope is selected with the `--scope` flag:

```bash
kubeseal --scope cluster-wide <secret.yaml >sealed-secret.json
```

It's also possible to request a scope via annotations in the input secret you pass to `kubeseal`:

- `sealedsecrets.bitnami.com/namespace-wide: "true"` -> for `namespace-wide`
- `sealedsecrets.bitnami.com/cluster-wide: "true"` -> for `cluster-wide`

The lack of any of such annotations means `strict` mode. If both are set, `cluster-wide` takes precedence.

> NOTE: Next release will consolidate this into a single `sealedsecrets.bitnami.com/scope` annotation.

## Installation

See https://github.com/bitnami-labs/sealed-secrets/releases for the latest release and detailed installation instructions.

Cloud platform specific notes and instructions:

- [GKE](docs/GKE.md)

### Controller

Once you deploy the manifest it will create the `SealedSecret` resource
and install the controller into `kube-system` namespace, create a service
account and necessary RBAC roles.

After a few moments, the controller will start, generate a key pair,
and be ready for operation. If it does not, check the controller logs.

#### Kustomize

The official controller manifest installation mechanism is just a YAML file.

In some cases you might need to apply your own customizations, like set a custom namespace or set some env variables.

`kubectl` has native support for that, see [kustomize](https://kustomize.io/).

#### Helm Chart

The Sealed Secrets helm chart is now officially supported and hosted in this GitHub repo.

```bash
helm repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
```

> NOTE: The versioning scheme of the helm chart differs from the versioning scheme of the sealed secrets project itself.

Originally the helm chart was maintained by the community and the first version adopted a major version of 1 while the
sealed secrets project itself is still at major 0.
This is ok because the version of the helm chart itself is not meant to be necessarily the version of the app itself.
However this is confusing, so our current versioning rule is:

1. The `SealedSecret` controller version scheme: 0.X.Y
2. The helm chart version scheme: 1.X.Y-rZ

There can be thus multiple revisions of the helm chart, with fixes that apply only to the helm chart without
affecting the static YAML manifests or the controller image itself.

> NOTE: The helm chart readme still contains a deprecation notice, but it no longer reflects reality and will be removed upon the next release.

> NOTE: The helm chart by default installs the controller with the name `sealed-secrets`, while the `kubeseal` command line interface (CLI) tries to access the controller with the name `sealed-secrets-controller`. You can explicitly pass `--controller-name` to the CLI:

```bash
kubeseal --controller-name sealed-secrets <args>
```

Alternatively, you can set `fullnameOverride` when installing the chart to override the name. Note also that `kubeseal` assumes that the controller is installed within the `kube-system` namespace by default. So if you want to use the `kubeseal` CLI without having to pass the expected controller name and namespace you should install the Helm Chart like this:

```bash
helm install sealed-secrets -n kube-system --set-string fullnameOverride=sealed-secrets-controller sealed-secrets/sealed-secrets
```

##### Helm Chart on a restricted environment

In some companies you might be given access only to a single namespace, not a full cluster.

One of the most restrictive environments you can encounter is:
- A `namespace` was allocated to you with some `service account`.
- You do not have access to the rest of the cluster, not even cluster CRDs.
- You may not even be able to create further service accounts or roles in your namespace.
- You are required to include resource limits in all your deployments.

Even with these restrictions you can still install the sealed secrets Helm Chart, there is only one pre-requisite:
- *The cluster must already have the sealed secrets CRDs installed*.

Once your admins installed the CRDs, if they were not there already, you can install the chart by preparing a YAML config file such as this:

```shell
serviceAccount:
  create: false
  name: {allocated-service-account}
rbac:
  create: false
  clusterRole: false
resources:
  limits:
    cpu: 150m
    memory: 256Mi
```

Note that:
- No service accounts are created, instead the one allocated to you will be used.
  - `{allocated-service-account}` is the name of the `service account` you were allocated on the cluster.
- No RBAC roles are created neither in the namespace nor the cluster.
- Resource limits must be specified.
  - The limits are samples that should work, but you might want to review them in your particular setup.

Once that file is ready, if you named it `config.yaml` you now can install the sealed secrets Helm Chart like this:

```shell
helm install sealed-secrets -n {allocated-namespace} sealed-secrets/sealed-secrets --skip-crds -f config.yaml
```

Where `{allocated-namespace}` is the name of the `namespace` you were allocated in the cluster.

### Kubeseal

#### Homebrew

The `kubeseal` client is also available on [homebrew](https://formulae.brew.sh/formula/kubeseal):

```bash
brew install kubeseal
```

#### MacPorts

The `kubeseal` client is also available on [MacPorts](https://ports.macports.org/port/kubeseal/summary):

```bash
port install kubeseal
```

#### Nixpkgs

The `kubeseal` client is also available on [Nixpkgs](https://search.nixos.org/packages?channel=unstable&show=kubeseal&from=0&size=50&sort=relevance&type=packages&query=kubeseal): (**DISCLAIMER**: Not maintained by bitnami-labs)

```bash
nix-env -iA nixpkgs.kubeseal
```

#### Linux

The `kubeseal` client can be installed on Linux, using the below commands:

```bash
KUBESEAL_VERSION='' # Set this to, for example, KUBESEAL_VERSION='0.23.0'
curl -OL "https://github.com/bitnami-labs/sealed-secrets/releases/download/v${KUBESEAL_VERSION:?}/kubeseal-${KUBESEAL_VERSION:?}-linux-amd64.tar.gz"
tar -xvzf kubeseal-${KUBESEAL_VERSION:?}-linux-amd64.tar.gz kubeseal
sudo install -m 755 kubeseal /usr/local/bin/kubeseal
```

If you have `curl` and `jq` installed on your machine, you can get the version dynamically this way. This can be useful for environments used in automation and such.

```
# Fetch the latest sealed-secrets version using GitHub API
KUBESEAL_VERSION=$(curl -s https://api.github.com/repos/bitnami-labs/sealed-secrets/tags | jq -r '.[0].name' | cut -c 2-)

# Check if the version was fetched successfully
if [ -z "$KUBESEAL_VERSION" ]; then
    echo "Failed to fetch the latest KUBESEAL_VERSION"
    exit 1
fi

curl -OL "https://github.com/bitnami-labs/sealed-secrets/releases/download/v${KUBESEAL_VERSION}/kubeseal-${KUBESEAL_VERSION}-linux-amd64.tar.gz"
tar -xvzf kubeseal-${KUBESEAL_VERSION}-linux-amd64.tar.gz kubeseal
sudo install -m 755 kubeseal /usr/local/bin/kubeseal
```

where `KUBESEAL_VERSION` is the [version tag](https://github.com/bitnami-labs/sealed-secrets/tags) of the kubeseal release you want to use. For example: `v0.18.0`.

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

## Upgrade

Don't forget to check the [release notes](RELEASE-NOTES.md) for guidance about
possible breaking changes when you upgrade the client tool
and/or the controller.

### Supported Versions
Currently, only the latest version of Sealed Secrets is supported for production environments.

### Compatibility with Kubernetes versions
The Sealed Secrets controller ensures compatibility with different versions of Kubernetes by relying on a stable Kubernetes API. Typically, Kubernetes versions above 1.16 are considered compatible. However, we officially support the [currently recommended Kubernetes versions](https://kubernetes.io/releases/). Additionally, versions above 1.24 undergo thorough verification through our CI process with every release.

## Usage

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

Note the `SealedSecret` and `Secret` must have **the same namespace and
name**. This is a feature to prevent other users on the same cluster
from re-using your sealed secrets. See the [Scopes](#scopes) section for more info.

`kubeseal` reads the namespace from the input secret, accepts an explicit `--namespace` argument, and uses
the `kubectl` default namespace (in that order). Any labels,
annotations, etc on the original `Secret` are preserved, but not
automatically reflected in the `SealedSecret`.

By design, this scheme *does not authenticate the user*. In other
words, *anyone* can create a `SealedSecret` containing any `Secret`
they like (provided the namespace/name matches). It is up to your
existing config management workflow, cluster RBAC rules, etc to ensure
that only the intended `SealedSecret` is uploaded to the cluster. The
only change from existing Kubernetes is that the *contents* of the
`Secret` are now hidden while outside the cluster.

### Managing existing secrets

If you want the Sealed Secrets controller to manage an existing `Secret`, you can annotate your `Secret` with the `sealedsecrets.bitnami.com/managed: "true"` annotation. The existing `Secret` will be overwritten when unsealing a `SealedSecret` with the same name and namespace, and the `SealedSecret` will take ownership of the `Secret` (so that when the `SealedSecret` is deleted the `Secret` will also be deleted).

### Patching existing secrets

> New in v0.23.0

There are some use cases in which you don't want to replace the whole `Secret` but just add or modify some keys from the existing `Secret`. For this, you can annotate your `Secret` with `sealedsecrets.bitnami.com/patch: "true"`. Using this annotation will make sure that secret keys, labels and annotations in the `Secret` that are not present in the `SealedSecret` won't be deleted, and those present in the `SealedSecret` will be added to the `Secret` (secret keys, labels and annotations that exist both in the `Secret` and the `SealedSecret` will be modified by the `SealedSecret`).

This annotation does not make the `SealedSecret` take ownership of the `Secret`. You can add both the `patch` and `managed` annotations to obtain the patching behavior while also taking ownership of the `Secret`.

### Seal secret which can skip set owner references

If you want `SealedSecret` and the `Secret` to be independent, which mean when you delete the `SealedSecret` the `Secret` won't disappear with it, then you have to annotate that Secret with the annotation `sealedsecrets.bitnami.com/skip-set-owner-references: "true"` ahead of applying the Usage steps. You still may also add `sealedsecrets.bitnami.com/managed: "true"` to your `Secret` so that your secret will be updated when `SealedSecret` is updated.

### Update existing secrets

If you want to add or update existing sealed secrets without having the cleartext for the other items,
you can just copy&paste the new encrypted data items and merge it into an existing sealed secret.

You must take care of sealing the updated items with a compatible name and namespace (see note about scopes above).

You can use the `--merge-into` command to update an existing sealed secrets if you don't want to copy&paste:

```bash
echo -n bar | kubectl create secret generic mysecret --dry-run=client --from-file=foo=/dev/stdin -o json \
  | kubeseal > mysealedsecret.json
echo -n baz | kubectl create secret generic mysecret --dry-run=client --from-file=bar=/dev/stdin -o json \
  | kubeseal --merge-into mysealedsecret.json
```

### Raw mode (experimental)

Creating temporary Secret with the `kubectl` command, only to throw it away once piped to `kubeseal` can
be a quite unfriendly user experience. We're working on an overhaul of the CLI experience. In the meantime,
we offer an alternative mode where kubeseal only cares about encrypting a value to stdout, and it's your responsibility to put it inside a `SealedSecret` resource (not unlike any of the other k8s resources).

It can also be useful as a building block for editor/IDE integrations.

The downside is that you have to be careful to be consistent with the sealing scope, the namespace and the name.

See [Scopes](#scopes)

`strict` scope (default):

```console
$ echo -n foo | kubeseal --raw --namespace bar --name mysecret
AgBChHUWLMx...
```

`namespace-wide` scope:

```console
$ echo -n foo | kubeseal --raw --namespace bar --scope namespace-wide
AgAbbFNkM54...
```
Include the `sealedsecrets.bitnami.com/namespace-wide` annotation in the `SealedSecret`
```yaml
metadata:
  annotations:
    sealedsecrets.bitnami.com/namespace-wide: "true"
```

`cluster-wide` scope:

```console
$ echo -n foo | kubeseal --raw --scope cluster-wide
AgAjLKpIYV+...
```
Include the `sealedsecrets.bitnami.com/cluster-wide` annotation in the `SealedSecret`
```yaml
metadata:
  annotations:
    sealedsecrets.bitnami.com/cluster-wide: "true"
```

### Validate a Sealed Secret

If you want to validate an existing sealed secret, `kubeseal` has the flag `--validate` to help you.

Giving a file named `sealed-secrets.yaml` containing the following sealed secret:

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

You can validate if the sealed secret was properly created or not:

```console
$ cat sealed-secrets.yaml | kubeseal --validate
```

In case of an invalid sealed secret, `kubeseal` will show:

```console
$ cat sealed-secrets.yaml | kubeseal --validate
error: unable to decrypt sealed secret
```

## Secret Rotation

You should always rotate your secrets. But since your secrets are encrypted with another secret,
you need to understand how these two layers relate to take the right decisions.

TL;DR:

> If a *sealing* private key is compromised, you need to follow the instructions below in "Early key renewal"
> section before rotating any of your actual secret values.
>
> SealedSecret key renewal and re-encryption features are **not a substitute** for periodical rotation of your actual secret values.

### Sealing key renewal

Sealing keys are automatically renewed every 30 days. Which means a new sealing key is created and appended to the set of active sealing keys the controller can use to unseal `SealedSecret` resources.

The most recently created sealing key is the one used to seal new secrets when you use `kubeseal` and it's the one whose certificate is downloaded when you use `kubeseal --fetch-cert`.

The renewal time of 30 days is a reasonable default, but it can be tweaked as needed
with the `--key-renew-period=<value>` flag for the command in the pod template of the `SealedSecret` controller. The `value` field can be given as golang
duration flag (eg: `720h30m`). Assuming that you've installed Sealed Secrets into the `kube-system` namespace, use the following command to edit the Deployment controller, and add the `--key-renew-period` parameter. Once you close your text editor, and the Deployment controller has been modified, a new Pod will be automatically created to replace the old Pod.

```
kubectl edit deployment/sealed-secrets-controller --namespace=kube-system
```

A value of `0` will deactivate automatic key renewal. Of course, you may have a valid use case for deactivating automatic sealing key renewal but experience has shown that new users often tend to jump to conclusions that they want control over key renewal, before fully understanding how sealed secrets work. Read more about this in the [common misconceptions](#common-misconceptions-about-key-renewal) section below.

> Unfortunately, you cannot use e.g. "d" as a unit for days because that's not supported by the Go stdlib. Instead of hitting your face with a palm, take this as an opportunity to meditate on the [falsehoods programmers believe about time](https://infiniteundo.com/post/25326999628/falsehoods-programmers-believe-about-time).

A common misunderstanding is that key renewal is often thought of as a form of key rotation, where the old key is not only obsolete but actually bad and that you thus want to get rid of it.
It doesn't help that this feature has been historically called "key rotation", which can add to the confusion.

Sealed secrets are not automatically rotated and old keys are not deleted
when new keys are generated. Old `SealedSecret` resources can be still decrypted (that's because old sealing keys are not deleted).

### User secret rotation

The *sealing key* renewal and SealedSecret rotation are **not a substitute** for rotating your actual secrets.

A core value proposition of this tool is:

> Encrypt your Secret into a SealedSecret, which *is* safe to store - even inside a public repository.

If you store anything in a version control storage, and in a public one in particular, you must assume
you cannot ever delete that information.

*If* a sealing key somehow leaks out of the cluster you must consider all your `SealedSecret` resources
encrypted with that key as compromised. No amount of sealing key rotation in the cluster or even re-encryption of existing SealedSecrets files can change that.

The best practice is to periodically rotate all your actual secrets (e.g. change the password) **and** craft new
`SealedSecret` resources with those new secrets.

But if the `SealedSecret` controller was not renewing the *sealing key* that rotation would be moot,
since the attacker could just decrypt the new secrets as well. Thus, you need to do both: periodically renew the sealing key and rotate your actual secrets!

### Early key renewal

If you know or suspect a *sealing key* has been compromised you should renew the key ASAP before you
start sealing your new rotated secrets, otherwise you'll be giving attackers access to your new secrets as well.

A key can be generated early by passing the current timestamp to the controller into a flag called `--key-cutoff-time` or an env var called `SEALED_SECRETS_KEY_CUTOFF_TIME`. The expected format is RFC1123, you can generate it with the `date -R` unix command.

### Common misconceptions about key renewal

Sealed secrets sealing keys are not access control keys (e.g. a password). They are more like the GPG key you might use to read encrypted mail sent to you. Let's continue with the email analogy for a bit:

Imagine you have reasons to believe your private GPG key might have been compromised. You'd have more to lose than to gain if the first thing you do is just delete your private key. All the previous emails sent with that key are no longer accessible to you (unless you have a decrypted copy of those emails), nor are new emails sent by your friends whom you have not yet managed to tell to use the new key.

Sure, the content of those encrypted emails is not secure, as an attacker might now be able to decrypt them, but what's done is done. Your sudden loss of the ability to read those emails surely doesn't undo the damage. If anything, it's worse because you no longer know for sure what secret the attacker got to know. What you really want to do is to make sure that your friend stops using your old key and that from now on all further communication is encrypted with a new key pair (i.e. your friend must know about that new key).

The same logic applies to SealedSecrets. The ultimate goal is to secure your actual "user" secrets. The "sealing" secrets are just a mechanism, an "envelope". If a secret is leaked there is no going back, what's done is done.

You first need to ensure that new secrets don't get encrypted with that old compromised key (in the email analogy above that's: create a new key pair and give all your friends your new public key).

The second logical step is to neutralize the damage, which depends on the nature of the secret. A simple example is a database password: if you accidentally leak your database password, the thing you're supposed to do is simply to change your database password (on the database; and revoke the old one!) *and* update the `SealedSecret` resource with the new password (i.e. running `kubeseal` again).

Both steps are described in the previous sections, albeit in a less verbose way. There is no shame in reading them again, now that you have a more in-depth grasp of the underlying rationale.

### Manual key management (advanced)

The `SealedSecret` controller and the associated workflow are designed to keep old sealing keys around and periodically add new ones. You should not delete old keys unless you know what you're doing.

That said, if you want you can manually manage (create, move, delete) *sealing keys*. They are just normal k8s secrets living in the same namespace where the `SealedSecret` controller lives (usually `kube-system`, but it's configurable).

There are advanced use cases that you can address by creative management of the sealing keys.
For example, you can share the same sealing key among a few clusters so that you can apply exactly the same sealed secret in multiple clusters.
Since sealing keys are just normal k8s secrets you can even use sealed secrets themselves and use a GitOps workflow to manage your sealing keys (useful when you want to share the same key among different clusters)!

Labeling a *sealing key* secret with anything other than `active` effectively deletes
the key from the `SealedSecret` controller, but it is still available in k8s for
manual encryption/decryption if need be.

**NOTE** `SealedSecret` controller currently does not automatically pick up manually created, deleted or relabeled sealing keys. An admin must restart the controller before the effect will apply.

### Re-encryption (advanced)

Before you can get rid of some old sealing keys you need to re-encrypt your SealedSecrets with the latest private key.

```bash
kubeseal --re-encrypt <my_sealed_secret.json >tmp.json \
  && mv tmp.json my_sealed_secret.json
```

The invocation above will produce a new sealed secret file freshly encrypted with
the latest key, without making the secrets leave the cluster to the client. You can then save that file
in your version control system (`kubeseal --re-encrypt` doesn't update the in-cluster object).

Currently, old keys are not garbage collected automatically.

It's a good idea to periodically re-encrypt your SealedSecrets. But as mentioned above, don't lull yourself in a false sense of security: you must assume the old version of the `SealedSecret` resource (the one encrypted with a key you think of as dead) is still potentially around and accessible to attackers. I.e. re-encryption is not a substitute for periodically rotating your actual secrets.

## Details (advanced)

This controller adds a new `SealedSecret` custom resource. The
interesting part of a `SealedSecret` is a base64-encoded
asymmetrically encrypted `Secret`.

The controller maintains a set of private/public key pairs as kubernetes
secrets. Keys are labeled with `sealedsecrets.bitnami.com/sealed-secrets-key`
and identified in the label as either `active` or `compromised`. On startup,
The sealed secrets controller will...

1. Search for these keys and add them to its local store if they are
labeled as active.
2. Create a new key
3. Start the key rotation cycle

### Crypto

More details about crypto can be found [here](docs/developer/crypto.md).

## Developing

Developing guidelines can be found [in the Developer Guide](docs/developer/README.md).

## FAQ

### Can I encrypt multiple secrets at once, in one YAML / JSON file?

Yes, you can! Drop as many secrets as you like in one file. Make sure to separate them via `---` for YAML and as extra, single objects in JSON.

### Will you still be able to decrypt if you no longer have access to your cluster?

No, the private keys are only stored in the Secret managed by the controller (unless you have some other backup of your k8s objects). There are no backdoors - without that private key used to encrypt a given SealedSecrets, you can't decrypt it. If you can't get to the Secrets with the encryption keys, and you also can't get to the decrypted versions of your Secrets live in the cluster, then you will need to regenerate new passwords for everything, seal them again with a new sealing key, etc.

### How can I do a backup of my SealedSecrets?

If you do want to make a backup of the encryption private keys, it's easy to do from an account with suitable access:

```bash
kubectl get secret -n kube-system -l sealedsecrets.bitnami.com/sealed-secrets-key -o yaml >main.key

echo "---" >> main.key
kubectl get secret -n kube-system sealed-secrets-key -o yaml >>main.key
```

> NOTE: You need the second statement only if you ever installed sealed-secrets older than version 0.9.x on your cluster.

> NOTE: This file will contain the controller's public + private keys and should be kept omg-safe!

> NOTE: After sealing key renewal you should recreate your backup. Otherwise, your backup won't be able to decrypt new sealed secrets.

To restore from a backup after some disaster, just put that secrets back before starting the controller - or if the controller was already started, replace the newly-created secrets and restart the controller:

* For Helm deployment:
    ```bash
    kubectl apply -f main.key
    kubectl delete pod -n kube-system -l app.kubernetes.io/name=sealed-secrets
    ```

* For deployment via `controller.yaml` manifest
    ```bash
    kubectl apply -f main.key
    kubectl delete pod -n kube-system -l name=sealed-secrets-controller
    ```

### Can I decrypt my secrets offline with a backup key?

While treating sealed-secrets as long term storage system for secrets is not the recommended use case, some people
do have a legitimate requirement for being able to recover secrets when the k8s cluster is down and restoring a backup into a new `SealedSecret` controller deployment is not practical.

If you have backed up one or more of your private keys (see previous question), you can use the `kubeseal --recovery-unseal --recovery-private-key file1.key,file2.key,...` command to decrypt a sealed secrets file.

### What flags are available for kubeseal?

You can check the flags available using `kubeseal --help`.

### How do I update parts of JSON/YAML/TOML/.. file encrypted with sealed secrets?

A kubernetes `Secret` resource contains multiple items, basically a flat map of key/value pairs.
SealedSecrets operate at that level, and does not care what you put in the values. In other words
it cannot make sense of any structured configuration file you might have put in a secret and thus
cannot help you update individual fields in it.

Since this is a common problem, especially when dealing with legacy applications, we do offer an [example](docs/examples/config-template) of a possible workaround.

### Can I bring my own (pre-generated) certificates?

Yes, you can provide the controller with your own certificates, and it will consume them.
Please check [here](docs/bring-your-own-certificates.md) for a workaround.

### How to use kubeseal if the controller is not running within the `kube-system` namespace?

If you installed the controller in a different namespace than the default `kube-system`, you need to provide this namespace
to the `kubeseal` commandline tool. There are two options:

1. You can specify the namespace via the command line option `--controller-namespace <namespace>`:

  ```bash
kubeseal --controller-namespace sealed-secrets <mysecret.json >mysealedsecret.json
```

2. Via the environment variable `SEALED_SECRETS_CONTROLLER_NAMESPACE`:

  ```bash
export SEALED_SECRETS_CONTROLLER_NAMESPACE=sealed-secrets
kubeseal <mysecret.json >mysealedsecret.json
```

### How to verify the images?

Our images are being signed using [cosign](https://github.com/sigstore/cosign). The signatures have been saved in our [GitHub Container Registry](https://ghcr.io/bitnami-labs/sealed-secrets-controller/signs).

> Images up to and including v0.20.2 were signed using Cosign v1. Newer images are signed with Cosign v2.

It is pretty simple to verify the images:

```bash
# export the COSIGN_VARIABLE setting up the GitHub container registry signs path
export COSIGN_REPOSITORY=ghcr.io/bitnami-labs/sealed-secrets-controller/signs

# verify the image uploaded in GHCR
cosign verify --key .github/workflows/cosign.pub ghcr.io/bitnami-labs/sealed-secrets-controller:latest

# verify the image uploaded in Dockerhub
cosign verify --key .github/workflows/cosign.pub docker.io/bitnami/sealed-secrets-controller:latest
```

### How to use one controller for a subset of namespaces

If you want to use one controller for more than one namespace, but not all namespaces, you can provide additional namespaces using the command line flag `--additional-namespaces=<namespace1>,<namespace2>,<...>`. Make sure you provide appropriate roles and rolebindings in the target namespaces, so the controller can manage the secrets in there.

## Community

- [#sealed-secrets on Kubernetes Slack](https://kubernetes.slack.com/messages/sealed-secrets)

Click [here](http://slack.k8s.io) to sign up to the Kubernetes Slack org.

### Related projects

- `kubeseal-convert`: [https://github.com/EladLeev/kubeseal-convert](https://github.com/EladLeev/kubeseal-convert)
- Visual Studio Code extension: [https://marketplace.visualstudio.com/items?itemName=codecontemplator.kubeseal](https://marketplace.visualstudio.com/items?itemName=codecontemplator.kubeseal)
- WebSeal: generates secrets in the browser: [https://socialgouv.github.io/webseal](https://socialgouv.github.io/webseal)
- HybridEncrypt TypeScript implementation: [https://github.com/SocialGouv/aes-gcm-rsa-oaep](https://github.com/SocialGouv/aes-gcm-rsa-oaep)
- [DEPRACATED] Sealed Secrets Operator: [https://github.com/disposab1e/sealed-secrets-operator-helm](https://github.com/disposab1e/sealed-secrets-operator-helm)
