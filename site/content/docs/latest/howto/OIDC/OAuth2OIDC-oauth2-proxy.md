# Deploying an auth proxy to access Kubeapps

## Using the chart

Kubeapps chart allows you to automatically deploy the proxy for you as a sidecar container if you specify the necessary flags. In a nutshell you need to enable the feature and set the client ID, secret and the IdP URL. The following examples use Google as the Identity Provider, modify the flags below to adapt them.

> If you are serving Kubeapps under a subpath (eg., "example.com/subpath") you will also need to set the `authProxy.oauthLoginURI` and `authProxy.oauthLogoutURI` flags, as well as the additional flag `--proxy-prefix`. For instance:

```bash
  # ... other OIDC flags
 --set authProxy.oauthLoginURI="/subpath/oauth2/login" \
 --set authProxy.oauthLogoutURI="/subpath/oauth2/logout" \
 --set authProxy.extraFlags="{<other flags>,--proxy-prefix=/subpath/oauth2}"\
```

**Example 1: Using the OIDC provider**

This example uses `oauth2-proxy`'s generic OIDC provider with Google, but is applicable to any OIDC provider such as Keycloak, Dex, Okta or Azure Active Directory etc. Note that the issuer url is passed as an additional flag here, together with an option to enable the cookie being set over an insecure connection for local development only:

```bash
helm install kubeapps bitnami/kubeapps \
  --namespace kubeapps \
  --set authProxy.enabled=true \
  --set authProxy.provider=oidc \
  --set authProxy.clientID=my-client-id.apps.googleusercontent.com \
  --set authProxy.clientSecret=my-client-secret \
  --set authProxy.cookieSecret=$(echo "not-good-secret" | base64) \
  --set authProxy.extraFlags="{--cookie-secure=false,--oidc-issuer-url=https://accounts.google.com}" \
```

**Example 2: Using a custom oauth2-proxy provider**

Some of the specific providers that come with `oauth2-proxy` are using OpenIDConnect to obtain the required IDToken and can be used instead of the generic oidc provider. Currently this includes only the GitLab, Google and LoginGov providers (see [OAuth2_Proxy's provider configuration](https://oauth2-proxy.github.io/oauth2-proxy/docs/configuration/overview) for the full list of OAuth2 providers). The user authentication flow is the same as above, with some small UI differences, such as the default login button is customized to the provider (rather than "Login with OpenID Connect"), or improved presentation when accepting the requested scopes (as is the case with Google, but only visible if you request extra scopes).

Here we no longer need to provide the issuer -url as an additional flag:

```bash
helm install kubeapps bitnami/kubeapps \
  --namespace kubeapps \
  --set authProxy.enabled=true \
  --set authProxy.provider=google \
  --set authProxy.clientID=my-client-id.apps.googleusercontent.com \
  --set authProxy.clientSecret=my-client-secret \
  --set authProxy.cookieSecret=$(echo "not-good-secret" | base64) \
  --set authProxy.extraFlags="{--cookie-secure=false}"
```

**Example 3: Authentication for Kubeapps on a GKE cluster**

Google Kubernetes Engine does not allow an OIDC IDToken to be used to authenticate requests to the managed API server, instead requiring the standard OAuth2 access token.
For this reason, when deploying Kubeapps on GKE we need to ensure that

- The scopes required by the user to interact with cloud platform are included, and
- The Kubeapps frontend uses the OAuth2 `access_key` as the bearer token when communicating with the managed Kubernetes API

Note that using the custom `google` provider here enables google to prompt the user for consent for the specific permissions requested in the scopes below, in a user-friendly way. You can also use the `oidc` provider but in this case the user is not prompted for the extra consent:

```bash
helm install kubeapps bitnami/kubeapps \
  --namespace kubeapps \
  --set authProxy.enabled=true \
  --set authProxy.provider=google \
  --set authProxy.clientID=my-client-id.apps.googleusercontent.com \
  --set authProxy.clientSecret=my-client-secret \
  --set authProxy.cookieSecret=$(echo "not-good-secret" | base64) \
  --set authProxy.scope="https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/cloud-platform" \
  --set authProxy.extraFlags="{--cookie-secure=false}" \
  --set frontend.proxypassAccessTokenAsBearer=true
```

## Manual deployment

In case you want to manually deploy the proxy, first you will create a Kubernetes deployment and service for the proxy. For the snippet below, you need to set the environment variables `AUTH_PROXY_CLIENT_ID`, `AUTH_PROXY_CLIENT_SECRET`, `AUTH_PROXY_DISCOVERY_URL` with the information from the IdP and `KUBEAPPS_NAMESPACE`.

```bash
export AUTH_PROXY_CLIENT_ID=<ID>
export AUTH_PROXY_CLIENT_SECRET=<SECRET>
export AUTH_PROXY_DISCOVERY_URL=<URL>
export AUTH_PROXY_COOKIE_SECRET=$(echo "not-good-secret" | base64)
kubectl create -n $KUBEAPPS_NAMESPACE -f - -o yaml << EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    name: kubeapps-auth-proxy
  name: kubeapps-auth-proxy
spec:
  replicas: 1
  selector:
    matchLabels:
      name: kubeapps-auth-proxy
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 1
    type: RollingUpdate
  template:
    metadata:
      labels:
        name: kubeapps-auth-proxy
    spec:
      containers:
      - args:
        - -provider=oidc
        - -client-id=$AUTH_PROXY_CLIENT_ID
        - -client-secret=$AUTH_PROXY_CLIENT_SECRET
        - -oidc-issuer-url=$AUTH_PROXY_DISCOVERY_URL
        - -cookie-secret=$AUTH_PROXY_COOKIE_SECRET
        - -cookie-refresh=2m
        - -upstream=http://localhost:8080/
        - -http-address=0.0.0.0:3000
        - -email-domain="*"
        - -pass-basic-auth=false
        - -pass-access-token=true
        - -pass-authorization-header=true
         - proxy-prefix=/oauth2
        image: bitnami/oauth2-proxy
        imagePullPolicy: IfNotPresent
        name: kubeapps-auth-proxy
---
apiVersion: v1
kind: Service
metadata:
  labels:
    name: kubeapps-auth-proxy
  name: kubeapps-auth-proxy
spec:
  ports:
  - name: http
    port: 3000
    protocol: TCP
    targetPort: 3000
  selector:
    name: kubeapps-auth-proxy
  sessionAffinity: None
  type: ClusterIP
EOF
```

The above is a sample deployment, depending on the configuration of the Identity Provider those flags may vary. For this example we use:

- `-client-id`, `-client-secret` and `-oidc-issuer-url`: Client ID, Secret and IdP URL as stated in the section above.
- `-upstream`: Internal URL for the `kubeapps` service.
- `-http-address=0.0.0.0:3000`: Listen in all the interfaces.
- `-proxy-prefix=/oauth2`: If you are serving Kubeapps under a subpath, with this parameter the default prefix can be changed.

**NOTE**: If the identity provider is deployed with a self-signed certificate (which may be the case for Keycloak or Dex) you will need to deactivate the TLS and cookie verification. For doing so you can add the flags `-ssl-insecure-skip-verify` and `--cookie-secure=false` to the deployment above. You can find more options for `oauth2-proxy` [here](https://oauth2-proxy.github.io/oauth2-proxy/docs/configuration/overview).

### Exposing the proxy

Once the proxy is in place and it's able to connect to the IdP we will need to expose it to access it as the main endpoint for Kubeapps (instead of the `kubeapps` service). We can do that with an Ingress object. Note that for doing so an [Ingress Controller](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-controllers) is needed. There are also other methods to expose the `kubeapps-auth-proxy` service, for example using `LoadBalancer` as type in a cloud environment. In case an Ingress is used, remember to modify the host `kubeapps.local` for the value that you want to use as a hostname for Kubeapps:

```bash
kubectl create -n $KUBEAPPS_NAMESPACE -f - -o yaml << EOF
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/connection-proxy-header: keep-alive
    nginx.ingress.kubernetes.io/proxy-read-timeout: "600"
  name: kubeapps
spec:
  rules:
  - host: kubeapps.local
    http:
      paths:
      - backend:
          serviceName: kubeapps-auth-proxy
          servicePort: 3000
        path: /
EOF
```
