# Kubeapps tutorials

This section of our documentation contains step-by-step tutorials to help outline what Kubeapps is capable of while helping you achieve specific aims, such as installing Kubeapps or managing different packages.

We hope our tutorials make as few assumptions as possible and are broadly accessible to anyone with an interest in Kubeapps. They should also be a good place to start learning about Kubeapps, how it works and what it's capable of.

| Tutorial                                                  | Description                                                                                                                                                                                                                                    |
| --------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [Getting started](./getting-started.md)                   | This guide walks you through the process of deploying Kubeapps for your cluster and installing an example application.                                                                                                                         |
| [Using an OIDC provider](./using-an-OIDC-provider.md)     | This guide walks you through the process of using an existing OAuth2 provider, including OIDC, to authenticate users within Kubeapps.                                                                                                          |
| [Managing Carvel packages](./managing-carvel-packages.md) | This guide walks you through the process of using Kubeapps for configuring and deploying [Packages](https://carvel.dev/kapp-controller/docs/latest/packaging/#package) and managing [Applications](https://carvel.dev/kapp/docs/latest/apps/). |
| [Managing Flux packages](./managing-flux-packages.md)     | This guide walks you through the process of using Kubeapps for configuring and deploying [Helm charts](https://helm.sh/) via [Flux](https://fluxcd.io/).                                                                                       |
| [Kubeapps on TKG](./kubeapps-on-tkg/README.md)            | This guide walks you through the process of configuring, deploying and using Kubeapps on a VMware Tanzu™ Kubernetes Grid™ cluster.                                                                                                             |

Alternatively, if you have a specific goal, but are already familiar with Kubeapps, take a look at our [How-to guides](../howto/README.md). These have more in-depth detail and can be applied to a broader set of features.

Take a look at our [Reference section](../reference/README.md) when you need to know design decisions, what functions the API supports, detailed developer guides, etc.

Finally, for a better understanding of Kubeapps architecture, our [Background section](../background/README.md) enable you to expand your knowledge.
