# Bring your own certificates

The controller generates its own certificates when is deployed for the first time, it also manages the renewal for you.
But you can also bring your own certificates so the controller can consume them as well.

The controller consumes certificates contained in any secret labeled with `sealedsecrets.bitnami.com/sealed-secrets-key=active`,
the secret has to live in the same namespace as the controller. There can be multiple such secrets.

Below you can find all the steps needed to create and consume your own certificates.

## Set your vars

```bash
export PRIVATEKEY="mytls.key"
export PUBLICKEY="mytls.crt"
export NAMESPACE="sealed-secrets"
export SECRETNAME="mycustomkeys"
```

## Generate a new RSA key pair (certificates)
* Note to change `-days` option to set certificate expiry date; default is 1 year
```bash
openssl req -x509 -days 365 -nodes -newkey rsa:4096 -keyout "$PRIVATEKEY" -out "$PUBLICKEY" -subj "/CN=sealed-secret/O=sealed-secret"
```

## Create a tls k8s secret, using your recently created RSA key pair

```bash
kubectl -n "$NAMESPACE" create secret tls "$SECRETNAME" --cert="$PUBLICKEY" --key="$PRIVATEKEY"
kubectl -n "$NAMESPACE" label secret "$SECRETNAME" sealedsecrets.bitnami.com/sealed-secrets-key=active
```

## Deleting the controller Pod is needed to pick the new keys

```bash
kubectl -n  "$NAMESPACE" delete pod -l name=sealed-secrets-controller
```

## See the new certificates (private keys) in the controller logs

```bash
kubectl -n "$NAMESPACE" logs -l name=sealed-secrets-controller

controller version: v0.12.1+dirty
2020/06/09 14:30:45 Starting sealed-secrets controller version: v0.12.1+dirty
2020/06/09 14:30:45 Searching for existing private keys
2020/06/09 14:30:45 registered private key secretname=sealed-secrets-key5rxd9
2020/06/09 14:30:45 registered private key secretname=mycustomkeys
2020/06/09 14:30:45 HTTP server serving on :8080
```

## Try your own certificates

Now you can try to seal a secret with your own certificate, instead of using the controller provided ones.

### Used your recently created public key to "seal" your secret

Use your own certificate (key) by using the `--cert` flag:

```bash
kubeseal --cert "./${PUBLICKEY}" --scope cluster-wide < mysecret.yaml | kubectl apply -f-
```

### We can see the secret has been unsealed succesfully

```bash
kubectl -n "$NAMESPACE" logs -l name=sealed-secrets-controller

controller version: v0.12.1+dirty
2020/06/09 14:30:45 Starting sealed-secrets controller version: v0.12.1+dirty
2020/06/09 14:30:45 Searching for existing private keys
2020/06/09 14:30:45 ----- sealed-secrets-key5rxd9
2020/06/09 14:30:45 ----- mycustomkeys
2020/06/09 14:30:45 HTTP server serving on :8080
2020/06/09 14:37:55 Updating test-namespace/mysecret
2020/06/09 14:37:55 Event(v1.ObjectReference{Kind:"SealedSecret", Namespace:"test-namespace", Name:"mysecret", UID:"f3a6c537-d254-4c06-b08f-ab9548f28f5b", APIVersion:"bitnami.com/v1alpha1", ResourceVersion:"20469957", FieldPath:""}): type: 'Normal' reason: 'Unsealed' SealedSecret unsealed successfully
```

**NOTE:**

`$PRIVATEKEY` is your private key, which is used by the controller to unseal your secret.
Don't share it with anyone you don't trust, and save it in a safe place!!
