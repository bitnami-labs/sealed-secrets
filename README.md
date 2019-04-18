# "Sealed Secrets" for Kubernetes

[![Build Status](https://travis-ci.org/bitnami-labs/sealed-secrets.svg?branch=master)](https://travis-ci.org/bitnami-labs/sealed-secrets)
[![Go Report Card](https://goreportcard.com/badge/github.com/bitnami-labs/sealed-secrets)](https://goreportcard.com/report/github.com/bitnami-labs/sealed-secrets)

**Problem:** "I can manage all my K8s config in git, except Secrets."

**Solution:** Encrypt your Secret into a SealedSecret, which *is* safe
to store - even to a public repository.  The SealedSecret can be
decrypted only by the controller running in the target cluster and
nobody else (not even the original author) is able to obtain the
original Secret from the SealedSecret.

<br>

## Installation

There are two parts to installation:
* [controller](#controller-installation) 
* [kubeseal](#kubeseal-installation)

See https://github.com/bitnami-labs/sealed-secrets/releases for the latest
release.

<br>

## Controller Installation

`controller.yaml` will install the `controller` into `kube-system` namespace, create a service account, necessary RBAC roles and a `SealedSecret` CRD.  
After a few moments, the `controller` will start, generate a key pair, and be ready for operation.  If it does not, check the `controller` logs.

The key certificate (public key portion) is used for sealing secrets, and will be printed to the `controller` log on startup.

```sh
# Note:  If installing on a GKE cluster, a ClusterRoleBinding may be needed to successfully deploy the controller in the final command.  Replace <your-email> with a valid email, and then deploy the cluster role binding:
$ USER_EMAIL=<your-email>
$ kubectl create clusterrolebinding $USER-cluster-admin-binding --clusterrole=cluster-admin --user=$USER_EMAIL

# get latest release version
$ release=$(curl --silent "https://api.github.com/repos/bitnami-labs/sealed-secrets/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')

# Download release
$ wget https://github.com/bitnami-labs/sealed-secrets/releases/download/$release/sealed-secrets_$release_$(uname)_$(uname -m).tar.gz

# Extract release
$ tar -zxvf sealed-secrets_$release_$(uname)_$(uname -m).tar.gz

# For clusters with RBAC
$ kubectl apply -f controller.yaml

# For clusters without RBAC
$ kubectl apply -f controller-norbac.yaml
```

### Saving key certificate - Recommended

The public cert can be commited to git and supplied as an argument to `kubeseal` instead of requiring cluster access. For example: `kubeseal --cert mycert.pem`

```sh
# Save public cert to file (if using port-forward or other method to expose the controller)
$ kubeseal --fetch-cert > mycert.pem
```

<br>

## Kubeseal Installation

MacOS: 
```sh
$ brew install kubeseal
```

Linux:
```sh
# get latest release version
$ release=$(curl --silent "https://api.github.com/repos/bitnami-labs/sealed-secrets/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')

# Download release
$ wget https://github.com/bitnami-labs/sealed-secrets/releases/download/$release/sealed-secrets_$release_$(uname)_$(uname -m).tar.gz

# Extract release
$ tar -zxvf sealed-secrets_$release_$(uname)_$(uname -m).tar.gz

# Install in path (if required)
$ sudo install -m 755 kubeseal /usr/local/bin/kubeseal
```

Windows:
- download latest [release](https://github.com/bitnami-labs/sealed-secrets/releases/latest)
- extract archive

<br>

## Client usage 

**WARNING**: A bug in the current version is limiting secrets to use the "opaque" type. If you need to use another secret type (eg: `kubernetes.io/dockerconfigjson`), please use kubeseal from release 0.5.1 until [#86](https://github.com/bitnami-labs/sealed-secrets/issues/86) and [#92](https://github.com/bitnami-labs/sealed-secrets/issues/92) are resolved.

`kubeseal` will fetch the certificate from the controller at runtime (requires secure access to the Kubernetes API server), which is convenient for interactive use.  
The recommended automation workflow is to store the certificate to local disk and use it offline with `kubeseal --cert mycert.pem`. See [Saving key certificate](#saving-key-certificate---recommended)


```sh
# Create a json/yaml-encoded Secret somehow:
# (note use of `--dry-run` - this is just a local file!)
$ kubectl create secret generic mysecret --dry-run --from-literal=foo=bar -o json >mysecret.json

# This is the important bit:
$ kubeseal <mysecret.json >mysealedsecret.json

# mysealedsecret.json is safe to upload to github, post to twitter, etc. Eventually:
$ kubectl create -f mysealedsecret.json

# Profit!
$ kubectl get secret mysecret
```

Note the `SealedSecret` and `Secret` must have *the same namespace and name*.  This is a feature to prevent other users on the same cluster from re-using your sealed secrets.  
`kubeseal` reads the namespace from the input secret, accepts an explicit `--namespace` arg, and uses the `kubectl` default namespace (in that order). Any labels, annotations, etc on the original `Secret` are preserved, but not automatically reflected in the `SealedSecret`.

By design, this scheme *does not authenticate the user*.  In other words, *anyone* can create a `SealedSecret` containing any `Secret` they like (provided the namespace/name matches).  
It is up to your existing config management workflow, cluster RBAC rules, etc to ensure that only the intended `SealedSecret` is uploaded to the cluster.  
The only change from existing Kubernetes is that the *contents* of the `Secret` are now hidden while outside the cluster.

<br>

## Details

This controller adds a new `SealedSecret` custom resource.  The
interesting part of a `SealedSecret` is a base64-encoded
asymmetrically encrypted `Secret`.

The controller looks for a cluster-wide private/public key pair on
startup, and generates a new 4096 bit (by default) RSA key pair if not found.  The key is
persisted in a regular `Secret` in the same namespace as the
controller.  The public key portion of this (in the form of a
self-signed certificate) should be made publicly available to anyone
wanting to use `SealedSecret`s with this cluster.  The certificate is
printed to the controller log at startup, and available via an HTTP
GET to `/v1/cert.pem` on the controller.

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

<br>

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md)

<br>

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

## Helm Chart
Sealed Secrets helm chart can be found on this [link](https://github.com/helm/charts/tree/master/stable/sealed-secrets)