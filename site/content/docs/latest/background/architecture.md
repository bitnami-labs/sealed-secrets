# The Kubeapps Overview

This document describes the Kubeapps architecture at a high level.

## Components

### Kubeapps dashboard

At the heart of Kubeapps is an in-cluster Kubernetes dashboard that provides you a simple browse and click experience for installing and managing Kubernetes applications packaged as Helm charts.

The dashboard is written in the JavaScript programming language and is developed using the React JavaScript library.

### Kubeops

Kubeops is the service in charge of communicating both with the Helm (v3) API and other k8s resources like AppRepositories or Secrets.
Check more details about the implementation in [this document](../reference/developer/kubeops.md). Note: this service is deprecated and in the process of being removed.

### Kubeapps-APIs

The Kubeapps APIs service provides a pluggable, gRPC-based API service enabling the Kubeapps UI (or other clients) to interact with different Kubernetes packaging formats in a consistent, extensible way.

You can read more details about the architecture, implementation and getting started in the [Kubeapps APIs developer documentation](../reference/developer/kubeapps-apis.md).

### Apprepository CRD and Controller

Chart repositories in Kubeapps are managed with a `CustomResourceDefinition` called `apprepositories.kubeapps.com`. Each repository added to Kubeapps is an object of type `AppRepository` and the `apprepository-controller` will watch for changes on those types of objects to update the list of available charts to deploy.

### `asset-syncer`

The `asset-syncer` component is a tool that scans a Helm chart repository and populates chart metadata in a database. This metadata is then served by the `assetsvc` component. Check more details about the implementation in this [document](../reference/developer/asset-syncer.md).

### `assetsvc`

The `assetsvc` component is a micro-service that creates an API endpoint for accessing the metadata for charts and other resources that's populated in a database. Check more details about the implementation in this [document](../reference/developer/asset-syncer.md).
