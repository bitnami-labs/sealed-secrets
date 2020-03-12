// Sealed Secrets Grafana Dashboards

{
  grafanaDashboards+:: {
    'sealed-secrets-controller.json': (import 'sealed-secrets-controller.json'),
  },
}
