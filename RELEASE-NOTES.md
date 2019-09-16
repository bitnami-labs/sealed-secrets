# Release Notes

Latest release:

[![](https://img.shields.io/github/release/bitnami-labs/sealed-secrets.svg)](https://github.com/bitnami-labs/sealed-secrets/releases/latest)

# v0.9.0 (DRAFT)

## Announcement

### Private key renewal

This release turns on an important security feature: a new private key will be now created every 30 days by default.
Existing sealed-secrets resources will still be decrypted until the keys are manually phased out.

You can read more about this feature and the problem of **secret rotation** and how it interacts with Sealed Secrets in this [README section](https://github.com/bitnami-labs/sealed-secrets#secret-rotation) or in the original GH issue #137.

This feature alone is not technically a breaking change for people who use the offline workflow with `kubeseal --cert`, since old keys are not rotated out automatically. Users would be required to update their offline certs only when they purge old keys manually (we might introduce automatic purging in the future).

That said, to reap the benefits of key renewal, users of the offline workflow are encouraged to update their offline certificates every time a new key is generated (by default every 30 days).

### Pre-v0.7.0 clients

If you are using kubeseal clients older than v0.7.0, please upgrade. Since this release the controller
will no longer accept the "v1" format of the encrypted "data" field and instead it will only support the
"encryptedData" field.

If you have old sealed secret resources lying around, you can easily upgrade them by invoking:

```bash
kubeseal --re-encrypt <old.yaml >new.yaml
```

### Update items

Since version v0.7.0 it was possible to update individual items in the `encryptedData` field of the Sealed Secret resource, but you had to manually copy&paste the encrypted items into an existing resource file. The required steps were never spelled out in the documentation and to be fair it always felt quite awkward.

Now `kubectl` has learned how to update an existing secret, whilist preserving the same general operation principles, namely staying out of the business of actually crafting the secret itself (`kubectl create secret ...` and its various flags like `--from-file`, `--from-literal`, etc). Example:

```bash
$ kubectl create secret generic mysecret --dry-run -o json --from-file=foo=/tmp/foo \
  | kubeseal >sealed.json
$ kubectl create secret generic mysecret --dry-run -o json --from-file=bar=/tmp/bar \
  | kubeseal --merge-into sealed.json
```

## Changelog

* Key rotation is enabled by default every 30 days (#236)
* You can now use env vars such as SEALED_SECRETS_FOO_BAR to customize the controller (#234)
* Disabling by default deprecated "v1" encrypted data format (used by pre-v0.7.0 clients) (#235)
* Fix RBAC rules for /v1/rotate and /v1/validate fixing #166 for good (#249)
* Implement the --merge-into command (#253)
* add the `-o` alias for `--format` (#261)

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/1?closed=1

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
