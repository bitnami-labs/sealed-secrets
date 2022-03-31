# Release Notes

Latest release:

[![](https://img.shields.io/github/release/bitnami-labs/sealed-secrets.svg)](https://github.com/bitnami-labs/sealed-secrets/releases/latest)

## v0.17.4

### Changelog

- Fix linter errors running golangci-lint ([#751](https://github.com/bitnami-labs/sealed-secrets/pull/751))([#771](https://github.com/bitnami-labs/sealed-secrets/pull/771))
- Added kubeseal support for darwin/arm64 ([#752](https://github.com/bitnami-labs/sealed-secrets/pull/752))
- Bump prometheus/client_golang dependency to avoid CVE-2022-21698 ([#783](https://github.com/bitnami-labs/sealed-secrets/pull/783))

## v0.17.3

### Changelog

- Unseal templates even when encryptedData is empty ([#653](https://github.com/bitnami-labs/sealed-secrets/pull/653))
- Add new RBAC rules to make Sealed Secret compatible with K8s environments with RBAC enabled ([#715](https://github.com/bitnami-labs/sealed-secrets/pull/715))
- Allow rencrypt/validate functionalities to work with named ports defined in the Sealed Secret service ([#726](https://github.com/bitnami-labs/sealed-secrets/pull/726))
- Fix verbose logging ([#727](https://github.com/bitnami-labs/sealed-secrets/pull/727))

## v0.17.2

### Changelog

- Fix issue fetching the certificate when the Sealed Secrets service has a named port ([#648](https://github.com/bitnami-labs/sealed-secrets/pull/648))
- Drop support for Go < 1.16 and bump client-go version ([#705](https://github.com/bitnami-labs/sealed-secrets/pull/705))

## v0.17.1

### Changelog

- Binaries to emit the proper version ([#683](https://github.com/bitnami-labs/sealed-secrets/pull/683))
- Re-enable publishing K8s manifests in GH releases ([#678](https://github.com/bitnami-labs/sealed-secrets/issues/678))

## v0.17.0

### Announcements

This release finally turns on the `update-status` feature flag that was introduced back in v0.12.0. The feature is considered stable (if it doesn't work for you, you can disable it by setting `SEALED_SECRETS_UPDATE_STATUS=0` in the controller manifest).

### Changelog

- Update rbac api version to `rbac.authorization.k8s.io/v1` ([#602](https://github.com/bitnami-labs/sealed-secrets/issues/602))
- Enable `--update-status` by default ([#583](https://github.com/bitnami-labs/sealed-secrets/pull/583))

## v0.16.0

### Changelog

- Add ability to template arbitrary data keys within resulting secrets ([#445](https://github.com/bitnami-labs/sealed-secrets/issues/445))
- Fix status CRD in controller.yaml (backport from helm chart) ([#567](https://github.com/bitnami-labs/sealed-secrets/issues/567))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/26?closed=1

## v0.15.0

This release contains a couple of fixes in the controller and manifests.

Notable mention: You can give the `--update-status` (also available as env var `SEALED_SECRETS_UPDATE_STATUS=1`) feature flag another try. We'll turn it on by default in ~~the next release~~ v0.17.0.

### Changelog

- Remove '{}' in CRD schema properties so that ArgoCD doesn't get confused ([#529](https://github.com/bitnami-labs/sealed-secrets/issues/529))
- Fix bug in status updates ([#223](https://github.com/bitnami-labs/sealed-secrets/issues/223))
- Add label-selector to filter Sealed Secrets ([#521](https://github.com/bitnami-labs/sealed-secrets/issues/521))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/28?closed=1

## v0.14.1

### Changelog

- Fixed `condition_info` prometheus metric disappearance ([#504](https://github.com/bitnami-labs/sealed-secrets/issues/504))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/27?closed=1

## v0.14.0

### Changelog

- Updated CustomResourceDefinition to apiextensions.k8s.io/v1 ([#490](https://github.com/bitnami-labs/sealed-secrets/issues/490))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/19?closed=1

## v0.13.1

### Changelog

- Make it easier to upgrade from ancient (pre v0.9.0) controllers ([#466](https://github.com/bitnami-labs/sealed-secrets/issues/466))
- Prometheus: add namespace to unseal error metric ([#463](https://github.com/bitnami-labs/sealed-secrets/issues/463))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/17?closed=1

## v0.12.6

# Announcements

This release contains a fix for [CVE-2020-14040](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2020-14040), which could have opened the possibility for an attacker to cause a DoS on the sealed-secret controller (provided the attacker can cause the controller to process a malicious sealed secret resource).

### Changelog

- Fix CVE-2020-14040 ([#456](https://github.com/bitnami-labs/sealed-secrets/issues/456))
- Don't require a namespace when using --raw and cluster-wide scope ([#451](https://github.com/bitnami-labs/sealed-secrets/issues/451))
- Unregister Prometheus Gauges associated to removed SealedSecrets conditions ([#422](https://github.com/bitnami-labs/sealed-secrets/issues/422))
- Add -f and -w flags as an alternative to stdin/out ([#439](https://github.com/bitnami-labs/sealed-secrets/issues/439))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/24?closed=1

## v0.12.5

### Changelog

- Add `condition_info` metric to expose SealedSecrets status ([#421](https://github.com/bitnami-labs/sealed-secrets/issues/421))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/23?closed=1

## v0.12.4

### Announcements

The binaries in this release have been rebuilt with the Go 1.14.3 toolchain. No other changes in binaries nor k8s manifests.

### Changelog

- Build with latest Go 1.14.x version ([#411](https://github.com/bitnami-labs/sealed-secrets/issues/411))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/22?closed=1

## v0.12.3

### Announcements

This release contains only a change in the `kubeseal` binary since v0.12.2. No controller nor k8s manifest changes.

### Changelog

- Fix `--merge-into` file permissions on Windows ([#407](https://github.com/bitnami-labs/sealed-secrets/issues/407))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/21?closed=1

## v0.12.2

### Announcements

This release contains important changes in manifests since v0.12.1.
It also contains a minor fix in kubeseal client.

Previously, users upgrading to v0.12.x from previous versions would experience:

```
The Deployment "sealed-secrets-controller" is invalid: spec.selector: Invalid value: v1.LabelSelector{MatchLabels:map[string]string{"app.kubernetes.io/managed-by":"jsonnet", "app.kubernetes.io/name":"kubeseal", "app.kubernetes.io/part-of":"kubeseal", "app.kubernetes.io/version":"v0.12.1", "name":"sealed-secrets-controller"}, MatchExpressions:[]v1.LabelSelectorRequirement(nil)}: field is immutable
```

This was caused by a bug in our official yaml manifests introduced in v0.12.0. Users of the Helm chart were unaffected.

By reverting this issue we're are going to cause the same bad experience for users who did perform a clean install of v0.12.x.
However, we believe such users are a minority.

### Changelog

- Revert "Add recommended labels" ([#404](https://github.com/bitnami-labs/sealed-secrets/issues/404))
- remove kubeconfig deps from recovery-unseal ([#394](https://github.com/bitnami-labs/sealed-secrets/issues/394))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/19?closed=1

## v0.12.1

### Announcements

This release contains changes in `kubeseal` and `controller` binaries but no changes in manifests since v0.12.0.

This release is a fixup release that turns off the status update feature introduced in v0.12.0. Several users have reported
a severe bug (an infinite feedback loop where the controller kept updating SealedSecrets and consuming lots of CPU).

In order to turn it back on you need to manually pass the `--update-status` flag to the *controller* (or pass the `SEALED_SECRETS_UPDATE_STATUS=1` env var)

### Changelog

- Make it easier to use --raw from stdin ([#386](https://github.com/bitnami-labs/sealed-secrets/issues/386))
- Disable status updates unless a feature flag is explicitly passed ([#388](https://github.com/bitnami-labs/sealed-secrets/issues/388))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/18?closed=1

## v0.12.0

### Announcements

This release contains changes in `kubeseal` and `controller` binaries as well as a minor change to the k8s manifest (see [#381](https://github.com/bitnami-labs/sealed-secrets/issues/381)); keep that in mind if you don't rely on the official k8s manifests, including the community-maintained Helm chart.

# Status field

Now the Sealed Secrets controller updates the `Status` field of the `SealedSecrets` resources.
This makes it easier for automation like ArgoCD to detect whether (and when) the controller has reacted to changes in the SealedSecret resources and produced a Secret. It also shows an error message in case it fails (many users are not familiar with k8s events and they may find it easier to see the error message in the status).

# Prometheus

The Sealed Secrets controller now exports prometheus metrics. See also [contrib/prometheus-mixin](contrib/prometheus-mixin) and `controller-podmonitor.yaml`.

### Changelog

- Update Status field ([#346](https://github.com/bitnami-labs/sealed-secrets/issues/346))
- Add prometheus metrics ([#177](https://github.com/bitnami-labs/sealed-secrets/issues/177))
- Upgrade k8s client-go to v0.16.8 ([#380](https://github.com/bitnami-labs/sealed-secrets/issues/380))
- kubeseal no longer emits empty `status: {}` field ([#383](https://github.com/bitnami-labs/sealed-secrets/issues/383))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/16?closed=1

## v0.11.0

### Announcements

This release contains only changes in kubeseal binary (no k8s manifest changes required).

### For those who choose the name and namespace after sealing the secret

Creating secrets with namespace-wide and cluster-wide scopes is now easier as it no longer requires manually adding annotations in the input Secret before passing it to `kubeseal`. This was often the root cause of many support requests. Now all you need to do is to:

```
$ kubeseal --scope namespace-wide <input-secret.yaml >output-sealed-secret.json
```

### Changelog

- Honour --scope flag ([#371](https://github.com/bitnami-labs/sealed-secrets/issues/371))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/15?closed=1

## v0.10.0

### Announcements

This release supports the ARM 32 bit and 64 bit architectures, both on the client and the controller sides.

We also end the silly streak of patch level releases that actually contained features. We'll try to bump the minor version on every release except true hotfixes.

### Changelog

- Provide multi-arch Container image for Sealed Secrets controller ([#349](https://github.com/bitnami-labs/sealed-secrets/issues/349))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/2?closed=1

## v0.9.8

### Announcements

This release contains only changes in Linux `kubeseal-arm` and `kubeseal-arm64` binaries. There are no changes in the docker images, nor in the `x86_64` binaries for any of the supported OS.

### Changelog

- Fix bad release of Linux ARM7 and ARM64 binaries ([#362](https://github.com/bitnami-labs/sealed-secrets/issues/362))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/14?closed=1

## v0.9.7

### Announcements

This release contains  changes in `kubeseal` and `controller` binaries as well as a minor change to the k8s manifest (see [#338](https://github.com/bitnami-labs/sealed-secrets/issues/338)); keep that in mind if you don't rely on the official k8s manifests, including the community-maintained Helm chart.

### Allow overwriting existing secrets

By default, the sealed-secrets controller doesn't unseal a SealedSecret over an existing Secret resource (i.e. a resource that has not been created by the sealed-secrets controller in the first place).

This is an important safeguard, not only to catch accidental overwrites due to typos etc, but also as a security measure: the sealed-secrets controller can create/update Secret resources even if the user who has the RBAC rights to create the SealedSecret resource doesn't have the right to create/update a Secret resource. We didn't want the sealed-secret controller to give its users more effective rights than what they would otherwise have without the sealed-secrets controller. A simple way to achieve that was permit only updates (overwrites) to Secret resources that were already owned by the sealed-secrets controller (which also seemed a sensible thing to do since it protects from accidental overwrites).

However, this behavior gets in the way when you're just starting to use SealedSecrets and want to migrate your existing Secrets into SealedSecrets.

You now can just annotate your `Secret`s with `sealedsecrets.bitnami.com/managed: true` thus indicating that they can be safely overwritten by the sealed-secrets controller. This doesn't loosen our security model since you'd have to have RBAC rights to annotate the existing secrets (e.g. with `kubectl annotate`) or you can ask your friendly admins to do it on your behalf.

### Changelog

- Release includes ARMv7 and ARM64 binaries (although no docker images yet) ([#173](https://github.com/bitnami-labs/sealed-secrets/issues/173))
- Set `fsGroup` to `nobody` in order to support `BoundServiceAccountTokenVolume` ([#338](https://github.com/bitnami-labs/sealed-secrets/issues/338))
- Add `--force-empty-data` flag to allow (un)sealing an empty secret ([#334](https://github.com/bitnami-labs/sealed-secrets/issues/334))
- Avoid forcing the default namespace when sealing a cluster-wide secret ([#323](https://github.com/bitnami-labs/sealed-secrets/issues/323))
- Introduce the `sealedsecrets.bitnami.com/managed: true` annotation which controls overwriting existing secrets ([#331](https://github.com/bitnami-labs/sealed-secrets/issues/331))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/13?closed=1

## v0.9.6

### Announcements

This release contains only changes in `kubeseal` and `controller` binaries (no k8s manifest changes required).

### Preliminary support for running multiple controllers

It always been possible in theory to run multiple controller instance in multiple namespaces,
each with their own sealing encryption keys and thus each able to unseal secrets intended for it.
However, doing so created a lot of noise in the logs, since each controller wouldn't know which
secrets are meant to be decryptable, but failed to decrypt, and which it ought to ignore.

Since v0.9.6 you can reduce this noise by setting the `--all-namespaces` flag to false (also via the env var `SEALED_SECRETS_ALL_NAMESPACES=false`).

### Changelog

- Give an option to search only the current namespace ([#316](https://github.com/bitnami-labs/sealed-secrets/issues/316))
- Support parsing multiple private keys in --recovery-private-key ([#325](https://github.com/bitnami-labs/sealed-secrets/issues/325))
- Add klog flags so we can troubleshoot k8s client ([#320](https://github.com/bitnami-labs/sealed-secrets/issues/320))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/12?closed=1

## v0.9.5

### Announcements

This release contains only changes in `kubeseal` binary (no k8s manifest changes required).

### Changelog

- Improve error reporting in case of missing kubeconfig when inferring namespace ([#313](https://github.com/bitnami-labs/sealed-secrets/issues/313))
- Teach kubeseal to decrypt using backed up secrets ([#312](https://github.com/bitnami-labs/sealed-secrets/issues/312))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/11?closed=1

## v0.9.4

### Announcements

This release contains only changes in `kubeseal` and `controller` binaries (no k8s manifest changes required).

### Changelog

- Remove tty warning in `--fetch-cert` (regression caused by #303 released in v0.9.3) ([#306](https://github.com/bitnami-labs/sealed-secrets/issues/306))
- Implement `--recovery-unseal` to help with disaster recovery ([#307](https://github.com/bitnami-labs/sealed-secrets/issues/307))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/10?closed=1

## v0.9.3

### Announcements

This release contains only changes in `kubeseal` and `controller` binaries (no k8s manifest changes required).

### Changelog

- Implement `--key-cutoff-time` ([#299](https://github.com/bitnami-labs/sealed-secrets/issues/299))
- Warn if stdin is a terminal ([#303](https://github.com/bitnami-labs/sealed-secrets/issues/303))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/9?closed=1

## v0.9.2

### Announcements

This release contains only changes in `kubeseal` and `controller` binaries (no k8s manifest changes required).

### Periodic key renewal and offline certificates

A few people have raised concerns of how will automatic key+certificate renewal affect the offline signing workflow.
First, a clarification: nothing changed. You can keep using your old certificates; it's just that if you do, you won't benefit from the additional security given from the periodic key renewal.

In order to simplify the workflow for those who do want to benefit from the key renewal, but at the same time
cannot access the target cluster (while not being completely offline), we offer a little feature that will help: `--cert` has learned to accept http(s) URLs. You can point it to a place where you serve up-to-date certificates for your clusters (tip/idea: you can expose the controller's cert.pem files with an Ingress).

### Changelog

- Accept URLs in `--cert` ([#281](https://github.com/bitnami-labs/sealed-secrets/issues/281))
- Improve logs/events in case of decryption error ([#274](https://github.com/bitnami-labs/sealed-secrets/issues/274))
- Reduce likelihood of name/namespace mismatch when using `--merge-into` ([#286](https://github.com/bitnami-labs/sealed-secrets/issues/286))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/8?closed=1

## v0.9.1

- Make manifests compatible with k8s 1.16.x ([#269](https://github.com/bitnami-labs/sealed-secrets/issues/269))
- Fix non-strict scopes with --raw ([#276](https://github.com/bitnami-labs/sealed-secrets/issues/276))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/7?closed=1

## v0.9.0

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

Now `kubectl` has learned how to update an existing secret, whilst preserving the same general operation principles, namely staying out of the business of actually crafting the secret itself (`kubectl create secret ...` and its various flags like `--from-file`, `--from-literal`, etc). Example:

```bash
$ kubectl create secret generic mysecret --dry-run -o json --from-file=foo=/tmp/foo \
  | kubeseal >sealed.json
$ kubectl create secret generic mysecret --dry-run -o json --from-file=bar=/tmp/bar \
  | kubeseal --merge-into sealed.json
```

### Changelog

- Doc improvements.
- Rename "key rotation" to "key renewal" since the terminology was confusing.
- Key renewal is enabled by default every 30 days ([#236](https://github.com/bitnami-labs/sealed-secrets/issues/236))
- You can now use env vars such as SEALED_SECRETS_FOO_BAR to customize the controller ([#234](https://github.com/bitnami-labs/sealed-secrets/issues/234))
- Disabling by default deprecated "v1" encrypted data format (used by pre-v0.7.0 clients) ([#235](https://github.com/bitnami-labs/sealed-secrets/issues/235))
- Fix RBAC rules for /v1/rotate and /v1/validate fixing #166 for good ([#249](https://github.com/bitnami-labs/sealed-secrets/issues/249))
- Implement the --merge-into command ([#253](https://github.com/bitnami-labs/sealed-secrets/issues/253))
- Add the `-o` alias for `--format` ([#261](https://github.com/bitnami-labs/sealed-secrets/issues/261))
- Add the `--raw` command for only encrypting single items ([#257](https://github.com/bitnami-labs/sealed-secrets/issues/257))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/1?closed=1

## v0.8.3

### Announcements

This release contains a fix for a possible secret leak that can happen when sealing existing secrets that have been retrieved from a cluster (e.g. with `kubectl get`) where they have been created with `kubectl apply` (as opposed to `kubectl create`).
This potential problem has been introduced v0.8.0 when kubeseal learned how to preserve annotations and labels.

Please check your existing sealed secret sources for any annotation `kubectl.kubernetes.io/last-applied-configuration`, because that annotation would contain your original secrets in clear.

This release strips this annotation (and a similar annotation created by the `kubcfg` tool)

### Changelog

Fixes in this release:

- Round-tripping secrets can leak clear-text in last-applied-configuration ([#227](https://github.com/bitnami-labs/sealed-secrets/issues/227))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/6?closed=1

## v0.8.2

Fixes in this release:

- Endless loop in controller on invalid base64 encrypted data bug ([#201](https://github.com/bitnami-labs/sealed-secrets/issues/201))
- Fix RBAC for /v1/cert.pem public key in isolated namespaces, removes most use cases for offline sealing with `--cert` ([#208](https://github.com/bitnami-labs/sealed-secrets/issues/208),[#166](https://github.com/bitnami-labs/sealed-secrets/issues/166))
- Accept and seal stringData into secret ([#221](https://github.com/bitnami-labs/sealed-secrets/issues/221))
- Fix a couple of blockers for enabling (still experimental) key rotation ([#185](https://github.com/bitnami-labs/sealed-secrets/issues/185), [#219](https://github.com/bitnami-labs/sealed-secrets/issues/219), [#218](https://github.com/bitnami-labs/sealed-secrets/issues/218))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/5?closed=1

## v0.8.1

Fixes in this release:

- Solve kubectl auth issues with clusters using `client.authentication.k8s.io/v1beta1` config by upgrading to client-go v12.0.0 ([#183](https://github.com/bitnami-labs/sealed-secrets/issues/183))
- Fix controller crash when writing logs due to read-only root FS ([#200](https://github.com/bitnami-labs/sealed-secrets/issues/200))

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/4?closed=1

## v0.8.0

The main improvements in this release are:

- support for annotations and labels ([#92](https://github.com/bitnami-labs/sealed-secrets/issues/92))
- support for secrets rotation opt-in ([#137](https://github.com/bitnami-labs/sealed-secrets/issues/137))
- fix bug with OwnerReferences handling ([#127](https://github.com/bitnami-labs/sealed-secrets/issues/127))
- EKS support; client-go version bump to release-7.0 ([#110](https://github.com/bitnami-labs/sealed-secrets/issues/110))
- Instructions to run on GKE when user is not cluster-admin ([#111](https://github.com/bitnami-labs/sealed-secrets/issues/111))
- Windows binary of kubeseal ([#85](https://github.com/bitnami-labs/sealed-secrets/issues/85))
- Internal codebase modernization (e.g. switch to Go modules)

The full Changelog is maintained in https://github.com/bitnami-labs/sealed-secrets/milestone/3?closed=1

Many thanks for all the folks who contributed to this release!

## v0.7.0

Big change for this release is the switch to **per-key encrypted values**.

- ("Keys" as in "object key/value", not as in "encryption key".  English is hard.)*
- Previously we generated a single big encrypted blob for each Secret, now we encrypt each value in the Secret separately, with the keys in plain text. This allows:
  - Existing keys can now be renamed and deleted without re-encrypting the value(s).
  - New keys/values can be added to the SealedSecret without re-encrypting (or even having access to!) the existing values.
  - Note that (as before) the encrypted values are still tied to the namespace/name of the enclosing Secret/SealedSecret, so can't be moved to another Secret.
   (The [cluster-wide annotation](https://github.com/bitnami-labs/sealed-secrets/blob/bda0af6a6a8abebc9ff359dd2e5e22d54cb40798/pkg/apis/sealed-secrets/v1alpha1/types.go#L16)  _does_ allow this, with the corresponding caveats, as before)
- The `kubeseal` tool does not yet have an option to output _just_ a single value, but you can safely mix+match the individual values from `kubeseal` output with an existing SealedSecret.  Improving `kubeseal` support for this feature is still an open action item.
- Existing/older "all-in-one" SealedSecrets are declared deprecated, but will continue to be supported by the controller for the foreseeable future.  New invocations of the `kubeseal` tool now produce per-key encrypted output - if you need to produce the older format, just use an older `kubeseal`.  Please raise a github issue if you have a use-case that requires supporting "all-in-one" SealedSecrets going forward.
- Note the CRD schema used for server-side validation in k8s >=1.9 has been temporarily removed, because it was unable to support the new per-key structure correctly (see [kubernetes/kubernetes#59485](https://github.com/kubernetes/kubernetes/issues/59485)).
- Huge thanks to @sullerandras for the code and his persistence in getting this merged!

## v0.6.0

- Support "cluster wide" secrets, that are not restricted to the original namespace
  - Set `sealedsecrets.bitnami.com/cluster-wide: "true"` annotation
  - Warning: cluster-wide SealedSecrets can be decrypted by anyone who can create a SealedSecret in your cluster
- Move to client-go v5.0
- Move to bitnami-labs github org
- Fix bug in schema validation for k8s 1.9

## v0.5.1

**Note:** this version moves TPR/CRD definition into a separate file.  To install, you need `controller.yaml` *and* either `sealedsecret-tpr.yaml` or `sealedsecret-crd.yaml`

- Add CRD definition and TPR->CRD migration documentation
- Add `kubeseal --fetch-cert` to dump server cert to stdout, for later offline use with `kubeseal --cert`
- Better sanitisation of input object to `kubeseal`

(v0.5.1 fixes a travis/github release issue with v0.5.0)

## v0.5.0

## v0.4.0

- controller: deployment security hardening: non-root uid and read-only rootfs
- `kubeseal`: Include oidc and gcp auth provider plugins
- `kubeseal`: Add support for YAML output

## v0.3.1

- Add `controller-norbac.yaml` to the release build. This is `controller.yaml` without RBAC rules and related service account - for environments where RBAC is not yet supported, [like Azure](https://github.com/Azure/acs-engine/issues/680).
- Fix missing controller RBAC ClusterRoleBinding in v0.3.0

## v0.3.0

Rename everything to better represent project scope.  Better to do this early (now) and apologies for the disruption.

- Rename repo and golang import path -> `bitnami/sealed-secrets`
- Rename cli tool -> `kubeseal`
- Rename `SealedSecret` apiGroup -> `bitnami.com`

## v0.2.1

- Fix invalid field `resourceName` in v0.2.0 controller.yaml (thanks @Globegitter)

## v0.2.0

- Client tool has better defaults, and can fetch the certificate automatically from the controller.
- Improve release process to include pre-built Linux and OSX x86-64 binaries.

## v0.1.0

Basic functionality is complete.

## v0.0.1

- Clean up controller.jsonnet
- Switch to quay.io (docker hub doesn't offer robot accounts??)
- Add deploy section to .travis.yml
