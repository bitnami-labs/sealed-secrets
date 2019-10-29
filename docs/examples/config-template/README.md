# Injecting secrets in config file templates

Kubernetes Secrets are very flexible and can be consumed in many ways.
Secret values can be passed to containers as environment variables or appear as regular files when mounting secret volumes.

Often users end up using the latter method just to wrap full configuration files into k8s secrets, just
because one or more fields in the config file happen to be secrets (e.g. a database password, or a session cookie encryption key).

Ideally you should avoid configuring your software that way, instead split your configuration from your secrets somehow. Also make sure you know about [12 Factor](https://www.12factor.net/).

That said, there are circumstances where you just have to provide such a file to your application (perhaps because it's a legacy app) and encrypting the whole configuration file in a single SealedSecrets item is problematic:

* You cannot easily update individual secret values (e.g. rotate your DB password), without first decrypting the whole configuration file.
* Since the whole configuration file is encrypted, it's hard to view, change (and review) non-secret parts of the config.

This example shows a possible approach to split the non-secret part of the config into a ConfigMap, and rely on a simple "Init Container" to merge the actual secret values back into it.

The example here includes an example Secret resource, which since you're reading this as part of the sealed-secrets documentation, you'd probably pass through `kubeseal` before checking it in; that said this approach is not limited to sealed-secrets.

This example uses the [gomplate](https://docs.gomplate.ca/) Go template based utility. We're not affiliated with it, it's just an example, you can use any other similar tool.
