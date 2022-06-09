# Access Control in Kubeapps

Before you configure the Kubernetes Role-Based-Access-Control for your authenticated users to access Kubeapps you will first need to establish how users will authenticate with the Kubernetes clusters on which Kubeapps operates.

## User Authentication in Kubeapps

By design, Kubeapps does not include a separate authentication layer but rather relies on the [supported mechanisms provided with Kubernetes itself](https://kubernetes.io/docs/reference/access-authn-authz/authentication/).

Each request to Kubeapps requires a token trusted by the Kubernetes API token in order to make
requests to the Kubernetes API server as the user. This ensures that a certain
user of Kubeapps is only permitted to view and manage applications to which they
have access (for example, within a specific namespace). If a user does not
have access to a particular resource, Kubeapps will display an error describing
the required roles to access the resource.

Two of the most common authentication strategies for providing a token identifying the user with Kubeapps are described below.

### OpenID Connect authentication

The most common and secure authentication for users to authenticate with the cluster (and therefore Kubeapps) is to use the built-in Kubernetes support for OpenID Connect. In this setup your clusters trust an OAuth2 provider such as Azure Active Directory, Google OpenID Connect or your own installation of the Dex auth provider. You can read more about [using an OIDC provider with Kubeapps](../tutorials/using-an-OIDC-provider.md).

### Service Account authentication

Alternatively, you can create Service Accounts for
Kubeapps users. This is not recommended for production use as Kubernetes service accounts are not designed to be used by users. That said, it is often a quick way to test or demo a Kubeapps installation without needing to configure OpenID Connect.

To create a Service Account for a user "example" in the "default" namespace, run the following:

```bash
kubectl create -n default serviceaccount example
```

To get the API token for this Service Account, run the following:

```bash
kubectl get -n default secret $(kubectl get -n default serviceaccount example -o jsonpath='{.secrets[].name}') -o go-template='{{.data.token | base64decode}}' && echo
```

## Assigning Kubeapps User Roles

The examples below demonstrate creating RBAC for a service account as it is easy to reproduce, but normally you would use the `--user` or `--group` arg rather than `--serviceaccount` when creating the role bindings for users.

You can install a set of preset Roles and ClusterRoles in your cluster
that you can bind to user or Service Accounts. Each Role and ClusterRole
pertains to a certain operation within Kubeapps. This documentation describes
the roles that should be applied to a user in order to perform operations within
Kubeapps.

### Applications

#### Read access to Applications within a namespace

In order to list and view Applications in a namespace, we will use the [default ClusterRole for viewing resources](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#user-facing-roles). Then we will bind that cluster role to our service account.

```bash
kubectl create -n default rolebinding example-view \
  --clusterrole=view \
  --serviceaccount default:example
```

This role should be enough to explore and discover the applications running in your cluster. It's certainly not enough for deploying new apps or managing package repositories.

#### Write access to Applications within a namespace

In order to create, update and delete Applications in a namespace, apply the`edit` ClusterRole in the desired namespace. The `edit` ClusterRole should be available in most Kubernetes distributions, you can find more information about that role [here](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#user-facing-roles).

```bash
kubectl create -n default rolebinding example-edit \
  --clusterrole=edit \
  --serviceaccount default:example
```

With the `edit` role, a user will be able to deploy and manage most of the applications available but it will still not be able to create new application repositories.

### Package Repositories

In order to manage Kubeapps package repositories, the `apprepositories-write` ClusterRole in the namespace Kubeapps is installed in is required. This cluster role include permissions to manage Kubernetes secrets in the namespace is installed (in order to manage package repository credentials) so treat it carefully:

```bash
export KUBEAPPS_NAMESPACE=kubeapps
kubectl create -n ${KUBEAPPS_NAMESPACE} rolebinding example-kubeapps-repositories-write \
  --clusterrole=kubeapps:${KUBEAPPS_NAMESPACE}:apprepositories-write \
  --serviceaccount default:example
```

> Note: There is also a cluster-role for just allowing people to read package repositories: `kubeapps:${KUBEAPPS_NAMESPACE}:apprepositories-read`.

The above command allows people to create package repositories in the Kubeapps namespace, these are called "Global Repositories" since they will be available in any namespace Kubeapps is available. On the other hand, it's also possible to create "Namespaced Repositories" that will be available just in a single namespace. For doing so, users need to have permissions to create package repositories in those namespaces. Read the next section to know how to create those roles.

### Assigning roles across multiple namespaces

To give permissions in multiple namespaces, simply create the same RoleBindings in each namespace you want to configure access for. For example, to give the "example" user permissions to manage Applications in the "example" namespace:

```bash
export KUBEAPPS_NAMESPACE=kubeapps
kubectl create -n example rolebinding example-kubeapps-applications-write \
  --clusterrole=kubeapps:${KUBEAPPS_NAMESPACE}:apprepositories-write \
  --serviceaccount default:example
```

If you want to give access for every namespace, simply create a ClusterRoleBinding instead of a RoleBinding. For example, we can give the "example" user permissions to manage Applications in _any_ namespace. Again, be careful applying this ClusterRole because it also allows to read and write Secrets:

```bash
export KUBEAPPS_NAMESPACE=kubeapps
kubectl create clusterrolebinding example-kubeapps-applications-write \
  --clusterrole=kubeapps:${KUBEAPPS_NAMESPACE}:apprepositories-write \
  --serviceaccount default:example
```

## Using a cluster-admin user (not recommended)

A simpler way to configure access for Kubeapps would be to give the user cluster-admin access (effectively disabling RBAC). This is not recommended, but
useful for quick demonstrations or evaluations.

```bash
kubectl create serviceaccount kubeapps-operator
kubectl create clusterrolebinding kubeapps-operator --clusterrole=cluster-admin --serviceaccount=default:kubeapps-operator
```
