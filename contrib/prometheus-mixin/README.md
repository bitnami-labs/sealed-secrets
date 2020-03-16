# Prometheus Mixin

A Prometheus mixin includes dashboards, recording rules and alerts provided to monitor
the application. For more details see 
[monitoring-mixins](https://github.com/monitoring-mixins/docs).

## Grafana dashboard

The [dashboard](./dashboards/sealed-secrets-controller.json) can be imported
standalone into Grafana. You may need to edit the datasource if you have
configured your Prometheus datasource with a different name.

## Using the mixin as jsonnet

See the [kube-prometheus](https://github.com/coreos/kube-prometheus#kube-prometheus)
project documentation for instructions on importing mixins.

## Generating YAML files and validating changes

Install the `jsonnet` dependencies:
```
$ go get github.com/google/go-jsonnet/cmd/jsonnet
$ go get github.com/google/go-jsonnet/cmd/jsonnetfmt
```

Generate yaml and run tests:
```
$ make
```
