# Sealed Secrets controller installation

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Assumptions and prerequisites](#assumptions-and-prerequisites)
- [Installing from Manifests](#installing-from-manifests)
  - [Installing in a GKE cluster](#installing-in-a-gke-cluster)
- [Installing the Helm Chart](#installing-the-helm-chart)
  - [Installing in an Openshift cluster](#installing-in-an-openshift-cluster)
- [Installing the Carvel package](#installing-the-carvel-package)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Assumptions and prerequisites

- You have access to an existing Kubernetes cluster (v1.16+).
- You have [`kubectl`](https://kubernetes.io/docs/tasks/tools/) command-line interface installed and configured to talk to your Kubernetes cluster.
- For the Helm installation, you have the [`helm`](https://helm.sh/docs/intro/install/) (v3.1.0+) command-line interface installed and configured to talk to your Kubernetes cluster.
- For the Carvel installation, you have the [`kapp`](https://carvel.dev/kapp/docs/latest/install/) command-line interface installed and configured to talk to your Kubernetes cluster.

The controller can be deployed using three different methods: direct yaml manifest installation, helm chart or carvel package.

## Installing from Manifests

Sealed secrets controller manifests are available from the [releases page](https://github.com/bitnami-labs/sealed-secrets/releases). You can choose the most convenient deployment for your cluster:

- `controller.yaml` Is a full manifest description of all the components required for the Sealed Secrets controller to operate. This includes Cluster role permissions and CRD definitions.
- `controller-norbac.yaml` Is a restricted version of the manifest descriptor. This version does not include CRDs nor Cluster roles.

To install the controller simply type:

```console
$ kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/{{VERSION}}/controller.yaml

role.rbac.authorization.k8s.io/sealed-secrets-service-proxier created
rolebinding.rbac.authorization.k8s.io/sealed-secrets-controller created
clusterrolebinding.rbac.authorization.k8s.io/sealed-secrets-controller created
serviceaccount/sealed-secrets-controller created
deployment.apps/sealed-secrets-controller created
customresourcedefinition.apiextensions.k8s.io/sealedsecrets.bitnami.com configured
rolebinding.rbac.authorization.k8s.io/sealed-secrets-service-proxier created
service/sealed-secrets-controller created
role.rbac.authorization.k8s.io/sealed-secrets-key-admin created
clusterrole.rbac.authorization.k8s.io/secrets-unsealer configured
```

Where `{{VERSION}}` is the Sealed Secrets latest version (i.e `v0.22.0`).

Once you deploy the manifest it will create the SealedSecret resource and install the controller into `kube-system` namespace, create a service account and necessary RBAC roles.

After a few moments, the controller will start, generate a key pair, and be ready for operation. If it does not, check the controller logs.

### Installing in a GKE cluster

Installing the controller on GKE clusters without admin rights might be problematic. For that, a `ClusterRoleBinding` will be needed to deploy the controller in the final command.  Replace `{{your-email}}` with a valid email, and then deploy the cluster role binding:

```bash
USER_EMAIL={{your-email}}
kubectl create clusterrolebinding $USER-cluster-admin-binding --clusterrole=cluster-admin --user=$USER_EMAIL
```

Please refer to the [GKE how-to](../howto/) for additional instructions on that platform.

## Installing the Helm Chart

The Sealed Secrets [Helm chart](https://helm.sh/) is officially supported and hosted in this GitHub repository.
```shell
helm repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
helm install sealed-secrets-controller sealed-secrets/sealed-secrets \
--set namespace=kube-system \
```

> The `kubeseal` CLI assumes that the controller is installed within the `kube-system` namespace by default with a deployment named `sealed-secrets-controller`. The above installation defines the same configuration to avoid unnecessary friction while using kubeseal.

### Installing in an Openshift cluster

Openshift installations will require some minor adjustments to comply with the standard Container Security Context restrictions:

```yaml
containerSecurityContext:
  enabled: true
  readOnlyRootFilesystem: true
  runAsNonRoot: true
  runAsUser: null
podSecurityContext:
```

## Installing the Carvel package

It is also possible to install Sealed Secrets as a [Carvel package](https://carvel.dev/kapp-controller/docs/v0.46.0/packaging/). To do so, you'll need to install `kapp-controller` in the target cluster and then deploy the needed `Package` and `PackageInstall` manifests.

```console
$ kapp deploy -a kc -f https://github.com/vmware-tanzu/carvel-kapp-controller/releases/latest/download/release.yml

$ kapp deploy -a sealed-secrets-carvel -f https://raw.githubusercontent.com/bitnami-labs/sealed-secrets/main/carvel/package.yaml
Changes

Namespace  Name                              Kind     Conds.  Age  Op      Op st.  Wait to    Rs  Ri
default    sealedsecrets.bitnami.com.2.10.0  Package  -       -    create  -       reconcile  -   -
...
Succeeded

$ kubectl get Package
NAME                               PACKAGEMETADATA NAME        VERSION   AGE
sealedsecrets.bitnami.com.2.10.0   sealedsecrets.bitnami.com   2.10.0    18s
```

Once the Package is available, it'll be necessary to execute the PackageInstall action, following the [carvel documentation](https://carvel.dev/kapp-controller/docs/v0.35.0/packaging-tutorial/#installing-a-package).
