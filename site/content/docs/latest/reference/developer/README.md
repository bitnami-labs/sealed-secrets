# The Kubeapps Components

### Kubeapps dashboard

The dashboard is the main UI component of the Kubeapps project. Written in JavaScript, the dashboard uses the React JavaScript library for the frontend.

Please refer to the [Kubeapps Dashboard Developer Guide](./dashboard.md) for the developer setup.

### Kubeapps APIs service

The Kubeapps APIs service is the main backend component of the Kubeapps project. Written in Go, the APIs service provides a pluggable gRPC service that is used to support different Kubernetes packaging formats.

See the [Kubeapps APIs Service Developer Guide](kubeapps-apis.md) for more information.

### kubeops

The `kubeops` component is a micro-service that creates an API endpoint for accessing the Helm API and Kubernetes resources. This service is in the process of being deprecated by the Kubeapps APIs service (which provides similar functionality in a generic, pluggable way) and is currently only used for some queries related to package repositories (and will be removed once they are available in the new Package Repositories API).

Please refer to the [Kubeapps Kubeops Developer Guide](kubeops.md) for the developer setup.

### asset-syncer

The `asset-syncer` component is a tool that scans a Helm chart repository and populates chart metadata in the database. This metadata is then served by the `kubeapps-apis` component.

Please refer to the [Kubeapps asset-syncer Developer Guide](asset-syncer.md) for the developer setup.
