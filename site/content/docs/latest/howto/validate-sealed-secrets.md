# How-to Validate existing Sealed Secrets

The `validate` Sealed Secrets feature is useful for ensuring the correctness of Sealed Secrets, especially when they need to be shared or used in various Kubernetes environments. By validating Sealed Secrets, you can verify that the encryption and decryption processes are functioning as expected and that the secrets are protected properly.

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
