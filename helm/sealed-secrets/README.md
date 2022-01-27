# Sealed Secrets

Sealed Secrets are "one-way" encrypted K8s Secrets that can be created by anyone, but can only be decrypted by the controller running in the target cluster recovering the original object.

## TL;DR

```console
$ helm repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
$ helm install my-release sealed-secrets/sealed-secrets
```

## Introduction

Bitnami charts for Helm are carefully engineered, actively maintained and are the quickest and easiest way to deploy containers on a Kubernetes cluster that are ready to handle production workloads.

This chart bootstraps a [Sealed Secret Controller](https://github.com/bitnami-labs/sealed-secrets) Deployment in [Kubernetes](http://kubernetes.io) using the [Helm](https://helm.sh) package manager.

Bitnami charts can be used with [Kubeapps](https://kubeapps.com/) for deployment and management of Helm Charts in clusters.

## Prerequisites

- Kubernetes 1.16+
- Helm 3.1.0

## Installing the Chart

To install the chart with the release name `my-release`:

```console
helm install my-release sealed-secrets/sealed-secrets
```

The command deploys the Sealed Secrets controller on the Kubernetes cluster in the default configuration. The [Parameters](#parameters) section lists the parameters that can be configured during installation.

> **Tip**: List all releases using `helm list`

## Uninstalling the Chart

To uninstall/delete the `my-release` deployment:

```console
helm delete my-release
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Parameters

### Common parameters

| Name               | Description                                             | Value |
| ------------------ | ------------------------------------------------------- | ----- |
| `kubeVersion`      | Override Kubernetes version                             | `""`  |
| `nameOverride`     | String to partially override sealed-secrets.fullname    | `""`  |
| `fullnameOverride` | String to fully override sealed-secrets.fullname        | `""`  |
| `namespace`        | Namespace where to deploy the Sealed Secrets controller | `""`  |
| `extraDeploy`      | Array of extra objects to deploy with the release       | `[]`  |


### Sealed Secrets Parameters

| Name                                              | Description                                                                          | Value                               |
| ------------------------------------------------- | ------------------------------------------------------------------------------------ | ----------------------------------- |
| `image.registry`                                  | Sealed Secrets image registry                                                        | `quay.io`                           |
| `image.repository`                                | Sealed Secrets image repository                                                      | `bitnami/sealed-secrets-controller` |
| `image.tag`                                       | Sealed Secrets image tag (immutable tags are recommended)                            | `v0.17.3`                           |
| `image.pullPolicy`                                | Sealed Secrets image pull policy                                                     | `IfNotPresent`                      |
| `image.pullSecrets`                               | Sealed Secrets image pull secrets                                                    | `[]`                                |
| `createController`                                | Specifies whether the Sealed Secrets controller should be created                    | `true`                              |
| `secretName`                                      | The name of an existing TLS secret containing the key used to encrypt secrets        | `sealed-secrets-key`                |
| `resources.limits`                                | The resources limits for the Sealed Secret containers                                | `{}`                                |
| `resources.requests`                              | The requested resources for the Sealed Secret containers                             | `{}`                                |
| `podSecurityContext.enabled`                      | Enabled Sealed Secret pods' Security Context                                         | `true`                              |
| `podSecurityContext.fsGroup`                      | Set Sealed Secret pod's Security Context fsGroup                                     | `65534`                             |
| `containerSecurityContext.enabled`                | Enabled Sealed Secret containers' Security Context                                   | `true`                              |
| `containerSecurityContext.readOnlyRootFilesystem` | Whether the Sealed Secret container has a read-only root filesystem                  | `true`                              |
| `containerSecurityContext.runAsNonRoot`           | Indicates that the Sealed Secret container must run as a non-root user               | `true`                              |
| `containerSecurityContext.runAsUser`              | Set Sealed Secret containers' Security Context runAsUser                             | `1001`                              |
| `podLabels`                                       | Extra labels for Sealed Secret pods                                                  | `{}`                                |
| `podAnnotations`                                  | Annotations for Sealed Secret pods                                                   | `{}`                                |
| `priorityClassName`                               | Sealed Secret pods' priorityClassName                                                | `""`                                |
| `affinity`                                        | Affinity for Sealed Secret pods assignment                                           | `{}`                                |
| `nodeSelector`                                    | Node labels for Sealed Secret pods assignment                                        | `{}`                                |
| `tolerations`                                     | Tolerations for Sealed Secret pods assignment                                        | `[]`                                |
| `updateStatus`                                    | Specifies whether the Sealed Secrets controller should update the status subresource | `true`                              |
| `keyrenewperiod`                                  | Specifies key renewal period. Default 30 days                                        | `""`                                |


### Traffic Exposure Parameters

| Name                       | Description                                                                                                                      | Value                    |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------ |
| `service.type`             | Sealed Secret service type                                                                                                       | `ClusterIP`              |
| `service.port`             | Sealed Secret service HTTP port                                                                                                  | `8080`                   |
| `service.nodePort`         | Node port for HTTP                                                                                                               | `""`                     |
| `service.annotations`      | Additional custom annotations for Sealed Secret service                                                                          | `{}`                     |
| `ingress.enabled`          | Enable ingress record generation for Sealed Secret                                                                               | `false`                  |
| `ingress.pathType`         | Ingress path type                                                                                                                | `ImplementationSpecific` |
| `ingress.apiVersion`       | Force Ingress API version (automatically detected if not set)                                                                    | `""`                     |
| `ingress.ingressClassName` | IngressClass that will be be used to implement the Ingress                                                                       | `""`                     |
| `ingress.hostname`         | Default host for the ingress record                                                                                              | `sealed-secrets.local`   |
| `ingress.path`             | Default path for the ingress record                                                                                              | `/v1/cert.pem`           |
| `ingress.annotations`      | Additional annotations for the Ingress resource. To enable certificate autogeneration, place here your cert-manager annotations. | `{}`                     |
| `ingress.tls`              | Enable TLS configuration for the host defined at `ingress.hostname` parameter                                                    | `false`                  |
| `ingress.selfSigned`       | Create a TLS secret for this ingress record using self-signed certificates generated by Helm                                     | `false`                  |
| `ingress.extraHosts`       | An array with additional hostname(s) to be covered with the ingress record                                                       | `[]`                     |
| `ingress.extraPaths`       | An array with additional arbitrary paths that may need to be added to the ingress under the main host                            | `[]`                     |
| `ingress.extraTls`         | TLS configuration for additional hostname(s) to be covered with this ingress record                                              | `[]`                     |
| `ingress.secrets`          | Custom TLS certificates as secrets                                                                                               | `[]`                     |
| `networkPolicy.enabled`    | Specifies whether a NetworkPolicy should be created                                                                              | `false`                  |


### Other Parameters

| Name                    | Description                                          | Value   |
| ----------------------- | ---------------------------------------------------- | ------- |
| `serviceAccount.create` | Specifies whether a ServiceAccount should be created | `true`  |
| `serviceAccount.labels` | Extra labels to be added to the ServiceAccount       | `{}`    |
| `serviceAccount.name`   | The name of the ServiceAccount to use.               | `""`    |
| `rbac.create`           | Specifies whether RBAC resources should be created   | `true`  |
| `rbac.labels`           | Extra labels to be added to RBAC resources           | `{}`    |
| `rbac.pspEnabled`       | PodSecurityPolicy                                    | `false` |


### Metrics parameters

| Name                                       | Description                                                                            | Value   |
| ------------------------------------------ | -------------------------------------------------------------------------------------- | ------- |
| `metrics.serviceMonitor.enabled`           | Specify if a ServiceMonitor will be deployed for Prometheus Operator                   | `false` |
| `metrics.serviceMonitor.namespace`         | Namespace where Prometheus Operator is running in                                      | `""`    |
| `metrics.serviceMonitor.labels`            | Extra labels for the ServiceMonitor                                                    | `{}`    |
| `metrics.serviceMonitor.annotations`       | Extra annotations for the ServiceMonitor                                               | `{}`    |
| `metrics.serviceMonitor.interval`          | How frequently to scrape metrics                                                       | `""`    |
| `metrics.serviceMonitor.scrapeTimeout`     | Timeout after which the scrape is ended                                                | `""`    |
| `metrics.serviceMonitor.metricRelabelings` | Specify additional relabeling of metrics                                               | `[]`    |
| `metrics.serviceMonitor.relabelings`       | Specify general relabeling                                                             | `[]`    |
| `metrics.dashboards.create`                | Specifies whether a ConfigMap with a Grafana dashboard configuration should be created | `false` |
| `metrics.dashboards.labels`                | Extra labels to be added to the Grafana dashboard ConfigMap                            | `{}`    |
| `metrics.dashboards.namespace`             | Namespace where Grafana dashboard ConfigMap is deployed                                | `""`    |


Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example,

```console
$ helm install my-release \
  --set resources.requests.cpu=25m \
    sealed-secrets/sealed-secrets
```

The above command sets the `resources.requests.cpu` parameter to `25m`.

Alternatively, a YAML file that specifies the values for the above parameters can be provided while installing the chart. For example,

```console
helm install my-release -f values.yaml sealed-secrets/sealed-secrets
```

## Using kubeseal

Install the kubeseal CLI by downloading the binary from [sealed-secrets/releases](https://github.com/bitnami-labs/sealed-secrets/releases).

Fetch the public key by passing the release name and namespace:

```bash
kubeseal --fetch-cert \
--controller-name=my-release \
--controller-namespace=my-release-namespace \
> pub-cert.pem
```

Read about kubeseal usage on [sealed-secrets docs](https://github.com/bitnami-labs/sealed-secrets#usage).

## Configuration and installation details

- In the case that **serviceAccount.create** is `false` and **rbac.create** is `true` it is expected for a ServiceAccount with the name **serviceAccount.name** to exist _in the same namespace as this chart_ before the installation.
- If **serviceAccount.create** is `true` there cannot be an existing service account with the name **serviceAccount.name**.
- If a secret with name **secretName** does not exist _in the same namespace as this chart_, then on install one will be created. If a secret already exists with this name the keys inside will be used.
- OpenShift: unset the runAsUser and fsGroup like this:

```yaml
securityContext:
  runAsUser:
  fsGroup:
```

## Troubleshooting

Find more information about how to deal with common errors related to Bitnami's Helm charts in [this troubleshooting guide](https://docs.bitnami.com/general/how-to/troubleshoot-helm-chart-issues).

## Upgrading

### To 2.0.0

A major refactoring of the chart has been performed to adopt several common practices for Helm charts. Upgrades from previous chart versions should work, however, the values structure suffered several changes and you'll have to adapt your custom values/parameters so they're aligned with the new structure. For instance, these are a couple of examples:

- `controller.create` renamed as `createController`.
- `securityContext.*` parameters are deprecated in favor of `podSecurityContext.*`, and `containerSecurityContext.*` ones.
- `image.repository` changed to `image.registry`/`image.repository`.
- `ingress.hosts[0]` changed to `ingress.hostname`.

Consult the [Parameters](#parameters) section to obtain more info about the available parameters.

[On November 13, 2020, Helm v2 support was formally finished](https://github.com/helm/charts#status-of-the-project), this new major version is no longer compatible with Helm v2.
