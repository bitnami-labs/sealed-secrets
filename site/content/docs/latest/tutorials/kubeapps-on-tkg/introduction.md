# Deploy and Configure Kubeapps on VMware Tanzu™ Kubernetes Grid™

## Introduction

[VMware Tanzu™ Kubernetes Grid™ (TKG)](https://tanzu.vmware.com/kubernetes-grid) is an enterprise-ready Kubernetes runtime that streamlines operations across a multi-cloud infrastructure. It can run both on-premise in vSphere and in the public cloud and includes signed and supported versions of open source applications to provide all the key services required for a production Kubernetes environment.

[Kubeapps](https://kubeapps.com/) provides a web-based dashboard to deploy, manage, and upgrade applications on a Kubernetes cluster. It is a one-time install that gives you a number of important benefits, including the ability to:

- browse and deploy packaged applications from public or private chart repositories;
- customize deployments through an intuitive, form-based user interface;
- upgrade, manage and delete the applications that are deployed in your Kubernetes cluster;

Kubeapps can be configured with public catalogs, such as the [VMware Marketplace™](https://marketplace.cloud.vmware.com/) catalog or the [Bitnami Application Catalog](https://bitnami.com/stacks/helm), or with private Helm repositories such as ChartMuseum or Harbor. It also integrates with [VMware Tanzu™ Application Catalog™ (TAC) for Tanzu™ Advanced](https://tanzu.vmware.com/application-catalog), which provides an enterprise-ready Helm chart catalog.

This guide walks you through the process of configuring, deploying and using Kubeapps on a VMware Tanzu™ Kubernetes Grid™ cluster. It covers the following tasks:

- Configuring an identity management provider in the cluster
- Integrating Kubeapps with the identity management provider
- Adjusting the Kubeapps user interface
- Configuring role-based access control in Kubeapps
- Deploying Kubeapps in the cluster
- Adding public and private repositories to Kubeapps: the public [VMware Marketplace™](https://marketplace.cloud.vmware.com/) repository and your private [VMware Tanzu™ Application Catalog™ for Tanzu™ Advanced](https://tanzu.vmware.com/application-catalog) repository
- Deploying applications through Kubeapps
- Listing, removing and managing applications through Kubeapps

## Intended Audience

This guide is intended for the following user roles:

- System administrators who want to install Kubeapps on a VMware Tanzu™ Kubernetes Grid™ cluster and use it to deploy and manage applications from the VMware Marketplace™ and the VMware Tanzu™ Application Catalog™ for Tanzu™ Advanced.
- Application administrators and developers who want to use Kubeapps to deploy and manage modern applications in a Kubernetes architecture.

In-depth knowledge of Kubernetes is not required.

## Assumptions and Prerequisites

This guide assumes that:

- You have a VMware Tanzu™ Kubernetes Grid™ v1.3.1 or later cluster. Check the [VMware Tanzu™ Kubernetes Grid™ 1.3 Documentation](https://docs.vmware.com/en/VMware-Tanzu-Kubernetes-Grid/1.3/vmware-tanzu-kubernetes-grid-13/GUID-index.html) for more information.
- You have access to the [VMWare Cloud Services Portal (CSP)](https://console.cloud.vmware.com/). If not, talk to your [VMware sales representative](https://www.vmware.com/company/contact_sales.html) to request access.
- You have access to, at a minimum, the Tanzu™ Application Catalog™ for Tanzu™ Advanced Demo environment. If not, reach out to your [VMware sales representative](https://www.vmware.com/company/contact_sales.html).
- You have the _kubectl_ CLI and the Helm v3.x package manager installed. Learn how to [install _kubectl_ and Helm v3.x](https://docs.bitnami.com/kubernetes/get-started-kubernetes/#step-3-install-kubectl-command-line).

## Steps

1. [Step 1: Configure an Identity Management Provider in the Cluster](./step-1.md)
2. [Step 2: Configure and Install Kubeapps](./step-2.md)
3. [Step 3: Add Application Repositories to Kubeapps](./step-3.md)
4. [Step 4: Deploy and Manage Applications with Kubeapps](./step-4.md)

Begin by [configuring an identity management provider](./step-1.md).
