# Using an OAuth2/OIDC Provider with Kubeapps

OpenID Connect (OIDC) is a simple identity layer on top of the OAuth 2.0 protocol which allows clients to verify the identity of a user based on the authentication performed by an authorization server, as well as to obtain basic profile information about the user.

A Kubernetes cluster can be configured to trust an external OIDC provider so that authenticated requests can be matched with defined RBAC. Additionally, some managed Kubernetes environments enable authenticating via plain OAuth2 (GKE).
This guide will explain how you can use an existing OAuth2 provider, including OIDC, to authenticate users within Kubeapps.

## Pre-requisites

For this guide, we assume that you have a Kubernetes cluster that is properly configured to use an OIDC Identity Provider (IdP) to handle the authentication to your cluster. You can read [more information about the Kubernetes API server's configuration options for OIDC](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#openid-connect-tokens). This allows that the Kubernetes API server itself to trust tokens from the identity provider. Some hosted Kubernetes services are already configured to accept access_tokens from their identity provider as bearer tokens (see GKE below).

Alternatively, if you do not have access to configure your cluster's API server, you can [install and configure Pinniped in your cluster to trust your identity provider and configure Kubeapps to proxy requests via Pinniped](../howto/OIDC/using-an-OIDC-provider-with-pinniped.md).

There are several Identity Providers (IdP) that can be used in a Kubernetes cluster. The steps of this guide have been validated using the following providers:

- [Keycloak](https://www.keycloak.org/): Open Source Identity and Access Management.
- [Dex](https://github.com/dexidp/dex): Open Source OIDC and OAuth 2.0 Provider with Pluggable Connectors.
- [Azure Active Directory](https://docs.microsoft.com/en-us/azure/active-directory/fundamentals/active-directory-whatis): Identity Provider that can be used for AKS.
- [Google OpenID Connect](https://developers.google.com/identity/protocols/OpenIDConnect): OAuth 2.0 for Google accounts.

When configuring the identity provider, you will need to ensure that the redirect URL for your Kubeapps installation is configured, which is your Kubeapps URL with the absolute path '/oauth2/callback'. For example, if I am deploying Kubeapps with TLS on the domain my-kubeapps.example.com, then the redirect URL will be `https://my-kubeapps.example.com/oauth2/callback`.

For Kubeapps to use an Identity Provider it's necessary to configure at least the following parameters:

- **Client ID**: Client ID of the IdP.
- **Client Secret**: (If configured) Secret used to validate the Client ID.
- **Provider name** (which can be oidc, in which case the OIDC Issuer URL is also required).
- **Cookie secret**: a 16, 24 or 32 byte base64 encoded seed string used to encrypt sensitive data (eg. `echo "not-good-secret" | base64`).

**Note**: Depending on the configuration of the Identity Provider more parameters may be needed.

## Configuration

Kubeapps uses [OAuth2 Proxy](https://github.com/oauth2-proxy/oauth2-proxy) to handle the OAuth2/OpenIDConnect authentication. The following sections explain how you can find the parameters above for some of the identity providers tested. If you have configured your cluster to use an Identity Provider you will already know some of these parameters. More detailed information can be found on the [OAuth2 Proxy Auth configuration page](https://oauth2-proxy.github.io/oauth2-proxy/docs/configuration/overview).

- [VMware Cloud Services](../howto/OIDC/OAuth2OIDC-VMware-cloud-services.md)
- [Azure Active Directory](../howto/OIDC/OAuth2OIDC-azure-active-directory.md)
- [Google OpenID Connect](../howto/OIDC/OAuth2OIDC-google-openid-connect.md)
- [Keycloak](../howto/OIDC/OAuth2OIDC-keycloak.md)
- [Dex](../howto/OIDC/OAuth2OIDC-dex.md)

For a complete worked example of this process on a specific Kubernetes environment, one of the Kubeapps developers has written a series detailing the installation of [Kubeapps on a set of VMware TKG clusters with OpenID Connect](https://liveandletlearn.net/post/kubeapps-on-tkg-management-cluster/).

## Deploying an auth proxy to access Kubeapps

The main difference in the authentication is that instead of accessing the Kubeapps service, we will be accessing an oauth2 proxy service that is in charge of authenticating users with the identity provider and injecting the required credentials in the requests to Kubeapps.

There are a number of available solutions for this use-case, like [keycloak-gatekeeper](https://github.com/keycloak/keycloak-gatekeeper) and [oauth2_proxy](https://github.com/oauth2-proxy/oauth2-proxy). For this guide we will use `oauth2_proxy` since it supports both OIDC and plain OAuth2 for many providers.

Once the proxy is accessible, you will be redirected to the identity provider to authenticate. After successfully authenticating, you will be redirected to Kubeapps and be authenticated with your user's OIDC token.

The next sections explain how you can deploy this proxy either using the Kubeapps chart or manually:

- [Using Kubeapps chart](../howto/OIDC/OAuth2OIDC-oauth2-proxy.md#using-the-chart)
- [Manual deployment](../howto/OIDC/OAuth2OIDC-oauth2-proxy.md#manual-deployment)

## Troubleshoothing

If you find after configuring your OIDC/OAuth2 setup following the above instructions, that although you can successfully authenticate with your provider you are nonetheless unable to login to Kubeapps but instead see a 403 or 401 request in the browser's debugger, then you will need to investigate _why_ the Kubernetes cluster is not accepting your credential.

Visit the [debugging auth failures when using OIDC](../howto/OIDC/OAuth2OIDC-debugging.md) page for more information.
