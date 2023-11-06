# Sealed Secrets Metrics

The Sealed Secrets Controller running in Kubernetes exposes Prometheus
metrics on `*:8081/metrics`.

These metrics enable operators to observe how it is performing. For example 
how many `SealedSecret` unseals have been attempted and how many errors may 
have occured due to RBAC permissions, wrong key, corrupted data, etc.

These metrics can be scraped by a Prometheus server and viewed in Prometheus,
displayed on a Grafana dashboard and/or trigger alerts to Slack/etc.

## Prometheus Mixin

A Prometheus mixin bundles all of the metric related concerns into a single
package for users of the application to consume.
Typically this includes dashboards, recording rules, alerts and alert logic
tests.

By creating a mixin, application maintainers and contributors to the project
can enshrine knowledge about operating the application and potential SLO's
that users may wish to use. 

For more details about this concept see the [monitoring-mixins](https://github.com/monitoring-mixins/docs)
project on GitHub.

## Scraping the metrics manually

After installing the Sealed Secrets Controller you can access the metrics via 
Kubernetes port-forward to your pod:

```
$ kubectl port-forward sealed-secrets-controller-6566dc69c6-lqr6x 8081 &
[1] 293283
```

Then query the metrics endpoint:
```
$ curl localhost:8081/metrics

<snip>
# HELP sealed_secrets_controller_build_info Build information.
# TYPE sealed_secrets_controller_build_info gauge
sealed_secrets_controller_build_info{revision="v0.12.1"} 0
# HELP sealed_secrets_controller_unseal_errors_total Total number of sealed secret unseal errors by reason
# TYPE sealed_secrets_controller_unseal_errors_total counter
sealed_secrets_controller_unseal_errors_total{reason="fetch"} 0
sealed_secrets_controller_unseal_errors_total{reason="status"} 0
sealed_secrets_controller_unseal_errors_total{reason="unmanaged"} 0
sealed_secrets_controller_unseal_errors_total{reason="unseal"} 0
sealed_secrets_controller_unseal_errors_total{reason="update"} 0
# HELP sealed_secrets_controller_unseal_requests_total Total number of sealed secret unseal requests
# TYPE sealed_secrets_controller_unseal_requests_total counter
sealed_secrets_controller_unseal_requests_total 86
```

## Scraping metrics with the Prometheus Operator

The [Prometheus Operator](https://github.com/coreos/prometheus-operator)
supports a couple of Kubernetes native scrape target `CustomResourceDefinitions`.

This project includes a [PodMonitor](../../controller-podmonitor.jsonnet
) CRD definition in jsonnet. To use this:

Compile jsonnet to yaml:
```
$ make controller-podmonitor.yaml 
kubecfg show -V CONTROLLER_IMAGE=docker.io/bitnami/sealed-secrets-controller:latest -V IMAGE_PULL_POLICY=Always -o yaml controller-podmonitor.jsonnet > controller-podmonitor.yaml.tmp
mv controller-podmonitor.yaml.tmp controller-podmonitor.yaml
```

Submit the `PodMonitor` CustomResourceDefinition to Kubernetes API:
```
$ kubectl apply -f controller-podmonitor.yaml
```

The Prometheus Operator will trigger a reload of Prometheus configuration and
you should see the Sealed Secrets Controller in your Prometheus UI under 
`Service Discovery` and `Targets`.

## Grafana dashboard

The [dashboard](./dashboards/sealed-secrets-controller.json) can be imported
standalone into Grafana. You may need to edit the datasource if you have
configured your Prometheus datasource with a different name.

## Using the mixin with kube-prometheus

See the [kube-prometheus](https://github.com/coreos/kube-prometheus#kube-prometheus)
project documentation for instructions on importing mixins.

## Using the mixin as raw YAML files

If you don't use the jsonnet based `kube-prometheus` project then you will need to
generate the raw yaml files for inclusion in your Prometheus installation.

Install the `jsonnet` dependencies:
```
$ go get github.com/google/go-jsonnet/cmd/jsonnet
$ go get github.com/google/go-jsonnet/cmd/jsonnetfmt
```

Generate yaml:
```
$ make
```
