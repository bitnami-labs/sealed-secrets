# Release Notes

Latest release:

[![](https://img.shields.io/github/release/bitnami-labs/sealed-secrets.svg)](https://github.com/bitnami-labs/sealed-secrets/releases/latest)

# v0.8.3

## Announcement

This release contains a fix for a possible secret leak that can happen when sealing existing secrets that have been retrieved from a cluster (e.g. with `kubectl get`) where they have been created with `kubectl apply` (as opposed to `kubectl create`).
This potential problem has been introduced v0.8.0 when kubeseal learned how to preserve annotations and labels.

Please check your existing sealed secret sources for any annotation `kubectl.kubernetes.io/last-applied-configuration`, because that annotation would contain your original secrets in clear.

This release strips this annotation (and a similar annotation created by the `kubcfg` tool)

## Changelog

Fixes in this release:
* Round-tripping secrets can leak cleartext in last-applied-configuration (#227)

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/6?closed=1

# v0.8.2

Fixes in this release:
* Endless loop in controller on invalid base64 encrypted data bug (#201)
* Fix RBAC for /v1/cert.pem public key in isolated namespaces, removes most use cases for offline sealing with `--cert` (#208,#166)
* Accept and seal stringData into secret (#221)
* Fix a couple of blockers for enabling (still experimental) key rotation (#185, #219, #218)

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/5?closed=1

# v0.8.1

Fixes in this release:
* Solve kubectl auth issues with clusters using `client.authentication.k8s.io/v1beta1` config by upgrading to client-go v12.0.0 (#183)
* Fix controller crash when writing logs due to read-only root FS (#200)

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/4?closed=1

# v0.8.0

The main improvements in this release are:
* support for annotations and labels (#92)
* support for secrets rotation opt-in (#137)
* fix bug with OwnerReferences handling (#127)
* EKS support; client-go version bump to release-7.0 (#110)
* Instructions to run on GKE when user is not cluster-admin (#111)
* Windows binary of kubeseal (#85)
* Internal codebase modernization (e.g. switch to Go modules)

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/3?closed=1

Many thanks for all the folks who contributed to this release!

# v0.7.0

Big change for this release is the switch to **per-key encrypted values**.
*("Keys" as in "object key/value", not as in "encryption key".  English is hard.)*
- Previously we generated a single big encrypted blob for each Secret, now we encrypt each value in the Secret separately, with the keys in plain text.
- This allows:
  - Existing keys can now be renamed and deleted without re-encrypting the value(s).
  - New keys/values can be added to the SealedSecret without re-encrypting (or even having access to!) the existing values.
  - Note that (as before) the encrypted values are still tied to the namespace/name of the enclosing Secret/SealedSecret, so can't be moved to another Secret.
   (The [cluster-wide annotation](https://github.com/bitnami-labs/sealed-secrets/blob/bda0af6a6a8abebc9ff359dd2e5e22d54cb40798/pkg/apis/sealed-secrets/v1alpha1/types.go#L16)  _does_ allow this, with the corresponding caveats, as before)
- The `kubeseal` tool does not yet have an option to output _just_ a single value, but you can safely mix+match the individual values from `kubeseal` output with an existing SealedSecret.  Improving `kubeseal` support for this feature is still an open action item.
- Existing/older "all-in-one" SealedSecrets are declared deprecated, but will continue to be supported by the controller for the foreseeable future.  New invocations of the `kubeseal` tool now produce per-key encrypted output - if you need to produce the older format, just use an older `kubeseal`.  Please raise a github issue if you have a use-case that requires supporting "all-in-one" SealedSecrets going forward.
- Note the CRD schema used for server-side validation in k8s >=1.9 has been temporarily removed, because it was unable to support the new per-key structure correctly (see kubernetes/kubernetes#59485).
- Huge thanks to @sullerandras for the code and his persistence in getting this merged!

# v0.6.0

- Support "cluster wide" secrets, that are not restricted to the original namespace
   - Set `sealedsecrets.bitnami.com/cluster-wide: "true"` annotation
   - Warning: cluster-wide SealedSecrets can be decrypted by anyone who can create a SealedSecret in your cluster
- Move to client-go v5.0
- Move to bitnami-labs github org
- Fix bug in schema validation for k8s 1.9

# v0.5.1

**Note:** this version moves TPR/CRD definition into a separate file.  To install, you need `controller.yaml` *and* either `sealedsecret-tpr.yaml` or `sealedsecret-crd.yaml`

- Add CRD definition and TPR->CRD migration documentation
- Add `kubeseal --fetch-cert` to dump server cert to stdout, for later offline use with `kubeseal --cert`
- Better sanitisation of input object to `kubeseal`

(v0.5.1 fixes a travis/github release issue with v0.5.0)

# v0.5.0

# v0.4.0

- controller: deployment security hardening: non-root uid and read-only rootfs
- `kubeseal`: Include oidc and gcp auth provider plugins
- `kubeseal`: Add support for YAML output

# v0.3.1

- Add `controller-norbac.yaml` to the release build. This is `controller.yaml` without RBAC rules and related service account - for environments where RBAC is not yet supported, [like Azure](https://github.com/Azure/acs-engine/issues/680).
- Fix missing controller RBAC ClusterRoleBinding in v0.3.0

# v0.3.0

Rename everything to better represent project scope.  Better to do this early (now) and apologies for the disruption.

- Rename repo and golang import path -> `bitnami/sealed-secrets`
- Rename cli tool -> `kubeseal`
- Rename `SealedSecret` apiGroup -> `bitnami.com`

# v0.2.1

- Fix invalid field `resourceName` in v0.2.0 controller.yaml (thanks @Globegitter)

# v0.2.0

- Client tool has better defaults, and can fetch the certificate automatically from the controller.
- Improve release process to include pre-built Linux and OSX x86-64 binaries.

# v0.1.0

Basic functionality is complete.

# v0.0.1

- Clean up controller.jsonnet
- Switch to quay.io (docker hub doesn't offer robot accounts??)
- Add deploy section to .travis.yml
