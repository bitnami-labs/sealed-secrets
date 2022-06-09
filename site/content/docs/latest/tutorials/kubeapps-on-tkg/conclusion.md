## Conclusion

This guide walked you through the process of configuring, deploying and using Kubeapps on a VMware Tanzu™ Kubernetes Grid™ cluster. It described the following steps:

- Configuring an identity management provider in the cluster
- Integrating Kubeapps with the identity management provider
- Adjusting the Kubeapps user interface
- Configuring role-based access control in Kubeapps
- Deploying Kubeapps in the cluster
- Adding public and private repositories to Kubeapps
- Deploying applications through Kubeapps
- Listing, removing and managing applications through Kubeapps

At the end of this guide, you should have everything you need to begin using Kubeapps productively on a VMware Tanzu™ Kubernetes Grid™ cluster.

In case of difficulties, reach out to the developers at [#kubeapps on Kubernetes Slack](https://kubernetes.slack.com/messages/kubeapps) (click [here](https://slack.k8s.io) to sign up). You can also [open a GitHub issue](https://github.com/vmware-tanzu/kubeapps/issues/new) to report problems or bugs.

## Useful Links

Learn more about the topics discussed in this guide using the links below.

### Background and Context

- [Enabling Identity Management in VMware Tanzu™ Kubernetes Grid™](https://docs.vmware.com/en/VMware-Tanzu-Kubernetes-Grid/1.3/vmware-tanzu-kubernetes-grid-13/GUID-mgmt-clusters-enabling-id-mgmt.html)
- [Installing Kubeapps in air-gapped environments](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/howto/offline-installation.md)
- [Syncing package repositories using webhooks](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/howto/syncing-apprepository-webhook.md)
- [Using Kubeapps to deploy in multiple clusters](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/howto/deploying-to-multiple-clusters.md)
- [Using Operators in Kubeapps](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/tutorials/operators.md)

### Step 1: Configure an Identity Management Provider in the Cluster

- [Using an OAuth2/OIDC provider](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/tutorials/using-an-OIDC-provider.md)
- [VMware Cloud Services as OIDC provider](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/tutorials/using-an-OIDC-provider.md#vmware-cloud-services)
- [Using an OIDC provider with Pinniped](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/howto/OIDC/using-an-OIDC-provider-with-pinniped.md)
- [JWTAuthenticator](https://pinniped.dev/docs/howto/configure-concierge-jwt/).

### Step 2: Configure and Install Kubeapps

- [Using values.yaml files in Helm Charts](https://helm.sh/docs/chart_template_guide/values_files/)
- [Configure Pinniped to trust the OIDC provider](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/howto/OIDC/using-an-OIDC-provider-with-pinniped.md#configure-pinniped-to-trust-your-oidc-identity-provider)
- [Configuring Kubeapps to proxy requests via Pinniped](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/howto/OIDC/using-an-OIDC-provider-with-pinniped.md#configuring-kubeapps-to-proxy-requests-via-pinniped)
- [Getting started with Kubeapps](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/tutorials/getting-started.md)
- [Adding new translations in Kubeapps](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/reference/translations/translate-kubeapps.md)

### Step 3: Add Application Repositories to Kubeapps

- [Adding an public application repository](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/howto/dashboard.md)
- [Consume Tanzu™ Application Catalog™ Helm Charts using Kubeapps](https://docs.vmware.com/en/VMware-Application-Catalog/services/main/GUID-using-consume-kubeapps.html)
- [Adding an private application repository](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/howto/private-app-repository.md)

### Step 4: Deploy and Manage Applications with Kubeapps

- [Using the Kubeapps Dashboard](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/howto/dashboard.md)
