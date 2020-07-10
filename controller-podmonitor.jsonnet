// Prometheus Service Monitor manifest

local controller = import 'controller.jsonnet';

controller {
  serviceMonitor: {
    apiVersion: 'monitoring.coreos.com/v1',
    kind: 'PodMonitor',
    metadata: {
      name: 'sealed-secrets-controller',
      namespace: $.namespace.metadata.namespace,
      labels: {
        name: 'sealed-secrets-controller',
      },
    },
    spec: {
      jobLabel: 'name',
      selector: {
        matchLabels: {
          name: 'sealed-secrets-controller',
        },
      },
      namespaceSelector: {
        matchNames: [
          $.namespace.metadata.namespace,
        ],
      },
      podMetricsEndpoints: [
        {
          port: 'http',
          interval: '30s',
        },
      ],
      sampleLimit: 1000,
    },
  },
}
