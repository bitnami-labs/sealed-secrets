# Supporting SSO without requiring K8s API server oidc configuration

We've recently demonstrated Kubeapps running in different environments configured with single-sign-on such that users can deploy to multiple worker clusters. Though this works very well, it is only possible because we can control the --oidc-\* arguments to the workload clusters' api server configuration, enabling the api server to trust the same OIDC identity provider (eg. Dex) that Kubeapps trusts.

In other environments, including but not restricted to Tanzu Mission Control, we do not have the ability to specify --oidc-\* arguments for clusters and therefore do not have a way to enable user credentials to be trusted by the Kubernetes api server. Often these environments do not allow customization of the cluster authentication. For example, the cluster authentication configured by TMC requires an API token (created via tmc login) to use the TMC API to generate a short-lived token (via tmc cluster generate-token-v2) which is used to authenticate with the K8s API server. This works for the kubectl integration but is not usable for user authentication in a web app resulting in a credential for use with the API server. The scenario is similar for other managed Kubernetes platforms.

Recently the [VMware Pinniped project](https://github.com/vmware-tanzu/pinniped) was announced which "allows cluster administrators to easily plug in external identity providers (IDPs) into Kubernetes clusters", importantly, without needing to do so at cluster creation time. This appears to provide an opportunity for a user credential obtained via a web login with an identity provider to be exchanged for a credential for use with the K8s API server, which Kubeapps requires. It is important to note that the current focus of the Pinniped team is for kubectl CLI integration supporting a variety of external identity providers - not web app authentication - but they are open to discussing future web app auth support.

## Design overview and discussion

You can read and comment on the design doc for [Kubeapps with Pinniped for Auth](https://docs.google.com/document/d/1WzDWQh1CDZ6fRg9Md-2l2l7JqVzFkZGACZA1WWog9AU/).
