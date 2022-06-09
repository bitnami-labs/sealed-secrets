# Kubeapps Dashboard Developer Guide

The dashboard is the main UI component of the Kubeapps project. Written in JavaScript, the dashboard uses the React JavaScript library for the frontend.

## Prerequisites

- [Git](https://git-scm.com/)
- [Node 12.x](https://nodejs.org/)
- [Yarn](https://yarnpkg.com)
- [Kubernetes cluster (v1.8+)](https://kubernetes.io/docs/setup/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
- [Docker CE](https://www.docker.com/community-edition)
- [Telepresence](https://telepresence.io)

_Telepresence is not a hard requirement, but is recommended for a better developer experience_

## Download the kubeapps source code

```bash
git clone --recurse-submodules https://github.com/vmware-tanzu/kubeapps $KUBEAPPS_DIR
```

The dashboard application source is located under the `dashboard/` directory of the repository.

```bash
cd $KUBEAPPS_DIR/dashboard
```

### Install Kubeapps in your cluster

Kubeapps is a Kubernetes-native application. To develop and test Kubeapps components we need a Kubernetes cluster with Kubeapps already installed. Follow the [Kubeapps installation guide](https://github.com/vmware-tanzu/kubeapps/blob/main/chart/kubeapps/README.md) to install Kubeapps in your cluster.

### Running the dashboard in development

[Telepresence](https://www.telepresence.io/) is a local development tool for Kubernetes microservices. As the dashboard is a service running in the Kubernetes cluster we use telepresence to proxy requests to the dashboard running in your cluster to your local development host.

First install the dashboard dependency packages:

```bash
yarn install
```

Next, create a `telepresence` shell to swap the `kubeapps-internal-dashboard` deployment in the `kubeapps` namespace, forwarding local port `3000` to port `8080` of the `kubeapps-internal-dashboard` pod.

```bash
telepresence --namespace kubeapps --method inject-tcp --swap-deployment kubeapps-internal-dashboard --expose 3000:8080 --run-shell
```

> **NOTE**: If you encounter issues getting this setup working correctly, please try switching the telepresence proxying method in the above command to `vpn-tcp`. Refer to [the telepresence docs](https://www.telepresence.io/reference/methods) to learn more about the available proxying methods and their limitations. If this doesn't work you can use the [Telepresence alternative](#telepresence-alternative).

Finally, launch the dashboard within the telepresence shell:

```bash
yarn run start
```

> **NOTE**: The commands above assume you install Kubeapps in the `kubeapps` namespace. Please update the file `dashboard/public/config.json` if you are using a different namespace.

#### Telepresence alternative

As an alternative to using [Telepresence](https://www.telepresence.io/) you can use the default [Create React App API proxy](https://create-react-app.dev/docs/proxying-api-requests-in-development/) functionality.

First add the desired host:port to the package.json:

```patch
-  }
+  },
+  "proxy": "http://127.0.0.1:8080"
```

> **NOTE**: Add the [proxy](https://github.com/vmware-tanzu/kubeapps/blob/main/dashboard/package.json#L176) `key:value` to the end of the `package.json`. For convenience, you can change the `host:port` values to meet your needs.

To use this a run Kubeapps per the [getting-started documentation](../../tutorials/getting-started.md#step-3-start-the-kubeapps-dashboard). This will start Kubeapps running on port `8080`.

Next you can launch the dashboard.

```bash
yarn run start
```

You can now access the local development server simply by accessing the dashboard as you usually would (e.g. doing a port-forward or accessing the Ingress URL).

#### Troubleshooting

In some cases, the 'Create React App' scripts keep listening on the 3000 port, even when you disconnect telepresence. If you see that `localhost:3000` is still serving the dashboard, even with your telepresence down, check if there is a 'Create React App' script process running (`ps aux | grep react`) and stop it.

### Running tests

Run the following command within the dashboard directory to start the test runner which will watch for changes and automatically re-run the tests when changes are detected.

```bash
yarn run test
```

> **NOTE**: macOS users may need to install watchman (<https://facebook.github.io/watchman/>).
