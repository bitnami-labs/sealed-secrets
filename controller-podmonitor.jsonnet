// Prometheus Service Monitor manifest

local controller = import 'controller.jsonnet';

controller {
  serviceMonitor: {
    apiVersion: 'monitoring.coreos.com/v1',
    kind: 'PodMonitor',
    metadata: {
      name: 'sealed-secrets-controller',
      namespace: $.namespace,
    },
    spec: {
      jobLabel: 'sealed-secrets-controller',
      selector: {
        matchLabels: {
          'app.kubernetes.io/name': 'sealed-secrets-controller',
        },
      },
      namespaceSelector: {
        matchNames: [
          $.namespace,
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
