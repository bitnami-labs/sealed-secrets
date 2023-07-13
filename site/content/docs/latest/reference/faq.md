# Frequently Asked Questions

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Will you still be able to decrypt if you no longer have access to your cluster?](#will-you-still-be-able-to-decrypt-if-you-no-longer-have-access-to-your-cluster)
- [How can I do a backup of my SealedSecrets?](#how-can-i-do-a-backup-of-my-sealedsecrets)
- [Can I decrypt my secrets offline with a backup key?](#can-i-decrypt-my-secrets-offline-with-a-backup-key)
- [What flags are available for kubeseal?](#what-flags-are-available-for-kubeseal)
- [How do I update parts of JSON/YAML/TOML/.. file encrypted with sealed secrets?](#how-do-i-update-parts-of-jsonyamltoml-file-encrypted-with-sealed-secrets)
- [Can I bring my own (pre-generated) certificates?](#can-i-bring-my-own-pre-generated-certificates)
- [How to use kubeseal if the controller is not running within the `kube-system` namespace?](#how-to-use-kubeseal-if-the-controller-is-not-running-within-the-kube-system-namespace)
- [How to verify the images?](#how-to-verify-the-images)
- [How to use one controller for a subset of namespaces](#how-to-use-one-controller-for-a-subset-of-namespaces)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Will you still be able to decrypt if you no longer have access to your cluster?

No, the private keys are only stored in the Secret managed by the controller (unless you have some other backup of your k8s objects). There are no backdoors - without that private key used to encrypt a given SealedSecrets, you can't decrypt it. If you can't get to the Secrets with the encryption keys, and you also can't get to the decrypted versions of your Secrets live in the cluster, then you will need to regenerate new passwords for everything, seal them again with a new sealing key, etc.

## How can I do a backup of my SealedSecrets?

If you do want to make a backup of the encryption private keys, it's easy to do from an account with suitable access:

```shell
kubectl get secret -n kube-system -l sealedsecrets.bitnami.com/sealed-secrets-key -o yaml >main.key

echo "---" >> main.key
kubectl get secret -n kube-system sealed-secrets-key -o yaml >>main.key
```

> NOTE: You need the second statement only if you ever installed sealed-secrets older than version 0.9.x on your cluster.

> NOTE: This file will contain the controller's public + private keys and should be kept omg-safe!

To restore from a backup after some disaster, just put that secrets back before starting the controller - or if the controller was already started, replace the newly-created secrets and restart the controller:

```shell
kubectl apply -f main.key
kubectl delete pod -n kube-system -l name=sealed-secrets-controller
```

## Can I decrypt my secrets offline with a backup key?

While treating sealed-secrets as long term storage system for secrets is not the recommended use case, some people
do have a legitimate requirement for being able to recover secrets when the k8s cluster is down and restoring a backup into a new `SealedSecret` controller deployment is not practical.

If you have backed up one or more of your private keys (see previous question), you can use the `kubeseal --recovery-unseal --recovery-private-key file1.key,file2.key,...` command to decrypt a sealed secrets file.

## What flags are available for kubeseal?

You can check the flags available using `kubeseal --help`.

## How do I update parts of JSON/YAML/TOML/.. file encrypted with sealed secrets?

A kubernetes `Secret` resource contains multiple items, basically a flat map of key/value pairs.
SealedSecrets operate at that level, and does not care what you put in the values. In other words
it cannot make sense of any structured configuration file you might have put in a secret and thus
cannot help you update individual fields in it.

Since this is a common problem, especially when dealing with legacy applications, we do offer an [example](docs/examples/config-template) of a possible workaround.

## Can I bring my own (pre-generated) certificates?

Yes, you can provide the controller with your own certificates, and it will consume them.
Please check [here](docs/bring-your-own-certificates.md) for a workaround.

## How to use kubeseal if the controller is not running within the `kube-system` namespace?

If you installed the controller in a different namespace than the default `kube-system`, you need to provide this namespace
to the `kubeseal` command line tool. There are two options:

1. You can specify the namespace via the command line option `--controller-namespace <namespace>`:

  ```shell
kubeseal --controller-namespace sealed-secrets <mysecret.json >mysealedsecret.json
```

2. Via the environment variable `SEALED_SECRETS_CONTROLLER_NAMESPACE`:

  ```shell
export SEALED_SECRETS_CONTROLLER_NAMESPACE=sealed-secrets
kubeseal <mysecret.json >mysealedsecret.json
```

## How to verify the images?

Our images are being signed using [cosign](https://github.com/sigstore/cosign). The signatures have been saved in our [GitHub Container Registry](https://ghcr.io/bitnami-labs/sealed-secrets-controller/signs).

> Images up to and including v0.20.2 were signed using Cosign v1. Newer images are signed with Cosign v2.

It is pretty simple to verify the images:

```console
# export the COSIGN_VARIABLE setting up the GitHub container registry signs path
$ export COSIGN_REPOSITORY=ghcr.io/bitnami-labs/sealed-secrets-controller/signs

# verify the image uploaded in GHCR
$ cosign verify --key .github/workflows/cosign.pub ghcr.io/bitnami-labs/sealed-secrets-controller:latest

Verification for ghcr.io/bitnami-labs/sealed-secrets-controller:latest --
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - Existence of the claims in the transparency log was verified offline
  - The signatures were verified against the specified public key
...

# verify the image uploaded in Dockerhub
$ cosign verify --key .github/workflows/cosign.pub docker.io/bitnami/sealed-secrets-controller:latest

Verification for index.docker.io/bitnami/sealed-secrets-controller:latest --
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - Existence of the claims in the transparency log was verified offline
  - The signatures were verified against the specified public key
...
```

## How to use one controller for a subset of namespaces

If you want to use one controller for more than one namespace, but not all namespaces, you can provide additional namespaces using the command line flag `--additional-namespaces=<namespace1>,<namespace2>,<...>`. Make sure you provide appropriate roles and rolebindings in the target namespaces, so the controller can manage the secrets in there.
