# Configuring Keycloak as an OIDC provider

This document explains how to configure Keycloak as an IDP + OIDC provider (check general information and pre-requisites for [using an OAuth2/OIDC Provider with Kubeapps](../../tutorials/using-an-OIDC-provider.md)).
It covers the installation and documentation for Kubeapps interacting with two Kubernetes clusters.

The installation used the [bitnami chart for Keycloak](https://github.com/bitnami/charts/tree/master/bitnami/keycloak) (version 12.0.4/2.4.8) and [bitnami chart for Kubeapps](https://github.com/bitnami/charts/tree/master/bitnami/kubeapps) (version 7.0.0/2.3.2)

# Keycloak Installation

## SSL

In order to support OIDC or OAuth, most servers and proxies require HTTPS. By default, the certificate created by the helm chart / Keycloak server is both invalid (error with `notBefore` attribute) and also based on a deprecated certificate version making it incompatible to use (i.e. is it based on Common Name instead of SAN and is rejected).

In this section, we will see how to configure Keycloak with its `auth.tls` enabled. Note that there are other options to configure TLS, for example via ingress TLS, either manually or using a cert manager. This section will focus on TLS at the server level.

Keycloak is a java-based server and is using JDK keystore/truststore. Both a keystore and a truststore must be created or the helm installation will fail. The keystore and truststore must then be installed on the cluster as a k8s Secret.

#### Step 1: create private key + certificate

The certificate must be a SAN certificate and should be capable of using wildcard alternative names. Keytool which is typically used for creating certificates does not support SANs with wildcards and is also using jks by default. We will use openssl instead.

To create the certificate, it is useful to create a openssl config file, something like the following (_certificate.config_):

```properties
[req]
default_bits        = 2048
distinguished_name  = req_dn
x509_extensions     = req_ext

[req_dn]
countryName                 = Country Name (2 letter code)
countryName_default         = US
stateOrProvinceName         = State or Province Name (full name)
stateOrProvinceName_default = CA
localityName                = Locality Name (eg, city)
localityName_default        = Campbell
organizationName            = Organization Name (eg, company)
organizationName_default    = vmware.com
commonName                  = Common Name (e.g. server FQDN or YOUR name)
commonName_default          = keycloak

[req_ext]
subjectAltName      = @alt_names

[alt_names]
DNS.1               = *.us-east-2.elb.amazonaws.com
```

The important section of the config is the `[req_ext]` extension section with alternate names. As the load balancer URL is created during installation, we use the wildcard mechanism so the final hostname can be verified by the certificates (here `DNS.1` illustrates a cluster on AWS). Other hostnames, e.g. localhost, can also be added following the numbers pattern (`DNS.x`).

The following command will generate the private key and certificate files:

```shell
openssl req \
      -newkey rsa:2048 -nodes -keyout certificate.key \
      -x509 -days 365 -config certificate.config -out certificate.crt
```

#### Step 2: create the keystore and truststore

Creating the keystore is done in two steps, starting by creating a pkcs12 keystore and then importing it using keytool (this will require a password, that we will need in the helm values.yaml):

```shell
openssl pkcs12 -export -in certificate.crt -inkey certificate.key -name keycloak -out certificate.p12

keytool -importkeystore -srckeystore certificate.p12  -srcstoretype pkcs12 -destkeystore keycloak-0.keystore.jks -alias keycloak
```

Then we need to create the truststore:

```shell
keytool -import -alias keycloak -file certificate.crt -keystore keycloak.truststore.jks
```

#### Step 3: create a k8s secret

The last step is to create a secret in the namespace where Keycloak will be installed.

Note that the names of the keystore and truststore matters and must be exactly as written here (make sure to rename them if the names have been changed in the commands in Step 2):

```shell
kubectl create secret generic keycloak-tls --from-file=./keycloak-0.keystore.jks  --from-file=./keycloak.truststore.jks
```

## Helm Install

To provide a default install, not many values must be provided in the values file - the values are mostly default passwords and the name of the secret created in Step 3 above.

Here is the `my-values.yaml` file that was applied when installing the Helm Chart:

```yaml
## Keycloak authentication parameters
## ref: https://github.com/bitnami/bitnami-docker-keycloak#admin-credentials
##
auth:
  ## Create administrator user on boot.
  ##
  createAdminUser: true

  ## Keycloak administrator user and password
  ##
  adminUser: admin
  adminPassword: Vmware!23

  ## Wildfly management user and password
  ##
  managementUser: manager
  managementPassword: Vmware!23

  ## TLS encryption parameters
  ## ref: https://github.com/bitnami/bitnami-docker-keycloak#tls-encryption
  ##
  tls:
    enabled: true

    ## Name of the existing secret containing the truststore and one keystore per Keycloak replica
    ##
    jksSecret: keycloak-tls

    ## Password to access the keystore when it's password-protected.
    ##
    keystorePassword: Vmware!23
    ## Password to access the truststore when it's password-protected.
    ##
    truststorePassword: Vmware!23
```

Then just deploy Keycloak either using Kubeapps UI or helm cli as follows:

```shell
helm install keycloak bitnami/keycloak --values my-values.yaml
```

# Keycloak Configuration

Follow the [Keycloak documentation](https://www.keycloak.org/documentation) to create and configure a new Realm to work with.

This section will focus on a few aspects to configure for the SSO scenario to work.

## Groups Claim

By default, there is no "groups" scope/claim. We will create a global client scope for groups.

In the admin console:

- Click "Client Scopes" from the left navigator menu
- Click on "Create" from the table (top right corner)
- Provide a name, ensure the protocol is set to "openid-connect" and that the option "Include in Token Scope" is on.

Once the client scope is created, you should be redirected to a page with several tabs. Navigate to the "Mappers" tab as we need to create a mapper to populate the value of the associated claim:

- Click on the "Mappers" tab
- Click on "Create" from the table to create a new mapper
- Configure:
  - Enter a name
  - Select "Group Membership" as the claim type
  - Enter "groups" as the token claim name
  - Ensure the "Full group path" is OFF
  - Keep the other knobs as ON
- Click ‘Save'

Note: if you navigate to "Client Scopes" and then select the tab "Default Client Scopes" you should be able to see the newly created "groups" scope in the "available client scopes" lists.

## Clients

In probably a very simplified view, Clients represent the application to be protected and accessed via SSO and OIDC. Here, the environment consisted of the Kubeapps web app and two Kubernetes clusters. So we need to create three clients.

### Cluster clients

For each of the two Kubernetes clusters, we will create a client as follows:

- Click "Clients" from the left navigator
- Click "Create" from the table
- Enter an "id" and Save (for example, `cluster1`and `cluster2` respectively)

Once created, configure the authentication as follows:

- Ensure the protocol is set to "openid-connect"
- Configure the "Access Type" to be "confidential". This will add a new "Credentials" tab from which you can get the client secret
- If you just want to use tokens to access the cluster, you can turn off the "Standard Flow Enabled". Keep this option if you plan to use a local browser login screen (e.g. if using pinniped cli).
- Ensure "Direct Access Grants Enabled" is enabled, as this is how we can get the tokens via API.
- Save

You then need to configure the client scopes that will be available:

- Click the "Client Scopes" tab
- Ensure the "email" scope is available either in the "Assigned Default Client Scopes" list or the "Assigned Optional Client Scopes" list
- The "groups" client scope should be available in the lists on the left. Add it either to the "Assigned Default Client Scopes" list or the "Assigned Optional Client Scopes" list.

### Kubeapps client

We need to first create the client:

- Click "Clients" from the left navigator
- Click "Create" from the table
- Enter an "id" and Save (e.g. `kubeapps`)

Once created, configure the authentication as follows:

- Ensure the protocol is set to "openid-connect"
- Configure the "Access Type" to be "confidential". This will add a new "Credentials" tab from which you can get the client secret
- Ensure "Standard Flow Enabled" is enabled, this is required for the login screen.
- "Direct Access Grants Enabled" can be disabled.
- In the "Valid Redirect URIs" field, enter "http://localhost:8000/\*" as a placeholder. We will need to revisit this field once we know the public hostname of kubeapps
- Save

As for the cluster clients, we need to configure the client scopes:

- Click the "Client Scopes" tab
- Ensure the "email" scope is available either in the "Assigned Default Client Scopes" list or the "Assigned Optional Client Scopes" list
- The "groups" client scope should be available in the lists on the left. Add it either to the "Assigned Default Client Scopes" list or the "Assigned Optional Client Scopes" list.

The last step is to configure the `kubeapps` client to be aware of the two cluster clients and be allowed to invoke them. There are different ways to configure Keycloak:

- Using automatic audience resolution. We haven't explored this method yet, therefore it won't be covered in this guide.
- Via Client Scopes: define the cluster clients as Client Scopes and add them to `kubeapps`.
- Via Mappers in the client: define a mapper attached to the `kubeapps` client that will inject the client ids in th audience claim.

#### Option #2

In this option, we create a client scope similar to how we created the groups client scope. This solution is better than solution #3 as the client ids are injected in the audience claim only if they were asked for in the scope request field.

- Click "Client Scopes" from the left navigator menu
- Click on "Create" from the table (top right corner)
- Provide a name (e.g. `cluster1-client`), ensure the protocol is set to "openid-connect" and that the option "Include in Token Scope" is on.
- Click the Mappers tab
- Click Create from the table to create a new mapper
  - Enter a name
  - Select the mapper type "Audience"
  - In "Included Client Audience" select the cluster client created above (e.g. `cluster1`)
  - Ensure "Add to ID token" is enabled
  - Save
- Repeat for the second cluster

Then in the `kubeapps` client:

- Navigate to the "Client Scope" tab
- The two new scopes created above should be available in the lists on the left. You can choose to add them to either the default list or the optional list.

#### Option #3

In this option, the claim is statically defined via a mapper similar to the one created in option #2.

- Navigate to the Mappers tab of the `kubeapps` client
- Click Create from the table
  - Enter a name
  - For Mapper Type select "Audience"
  - In "Included Client Audience" select the cluster client(e.g. `cluster1`)
  - Ensure "Add to ID token" is enabled
  - Save
- Repeat for the second cluster

The two client ids will be injected in the audience claim automatically.

## Users

Users are intuitive to create. But they must be configured with a "verified" email address.

The oauth proxy used in kubeapps requires email as the username. Furthermore, if the email is not marked as verified, JWT validation will fail and authentication will fail.

In order to test multiple users with different levels of authorization, it is useful to create them with multiple dummy email addresses. This can be done by ensuring that when the user is created, the field "email verified" is ON (skipping an actual email verification workflow).

# Kubeapps Installation

## Helm Install

Few changes are required to values.yaml for the helm installation:

- The `frontend` service type is set to LoadBalancer so we can have a public hostname for the callback. Using an ingress could be another alternative.
- The auth proxy must be configured. Here we will be using the default one.
- the `provider` field must be set to oidc
- The `clientID` and `clientSecret` field values can be retrieved from the `kubeapps` client in Keycloak
- the flag `--oidc-issuer-url` is the url to the Keycloak realm

The following excerpt shows the relevant values used in values.yaml when installing the Helm Chart:

```yaml
## Frontend parameters
##
frontend:
  service:
    type: LoadBalancer

# Auth Proxy configuration for OIDC support
# ref: https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/tutorials/using-an-OIDC-provider.md
authProxy:
  ## Set to true if Kubeapps should configure the OAuth login/logout URIs defined below.
  #
  enabled: true

  ## Skip the Kubeapps login page when using OIDC and directly redirect to the IdP
  ##
  skipKubeappsLoginPage: false

  ## Mandatory parameters for the internal auth-proxy.
  ##    those are the client id and secret from the oidc provider
  provider: oidc
  clientID: kubeapps
  clientSecret: 5b824b57-dc17-4ac8-8043-947d5edcfb03
 extraFlags:
  ## cookieSecret is used by oauth2-proxy to encrypt any credentials so that it requires
  ## no storage. Note that it must be a particular number of bytes. Recommend using the
  ## following to generate a cookieSecret as per the oauth2 configuration documentation
  ## at https://pusher.github.io/oauth2_proxy/configuration :
  ## python -c 'import os,base64; print base64.urlsafe_b64encode(os.urandom(16))'
  cookieSecret: Y29va2llLXNlY3JldC0xNg==
  ## Use "example.com" to restrict logins to emails from example.com
  emailDomain: "*"
  ## Additional flags for oauth2-proxy
  ##
    - --ssl-insecure-skip-verify
    - --cookie-secure=false
    - --scope=openid email groups
    - --oidc-issuer-url=https://<xxx>.us-east-2.elb.amazonaws.com/auth/realms/AWS
```

## Configuration

Once Kubeapps is installed and the load balancer is ready, we need to go back to Keycloak to configure the callback URL:

- Navigate to the `kubeapps` Client
- In the "Valid Redirect URIs" enter the callback URL for Kubeapps. It will be of the form "http://`<hostname>`/oauth2/callback" (where `<hostname>` is the load balancer hostname)

## Users

Users created in Keycloak will be authenticated but they will not have access to the cluster resources by default. Make sure to create role bindings to users and/or groups in both clusters.
