{
  prometheusAlerts+:: {
    groups+: [{
      name: 'sealed-secrets',
      rules: [
        // SealedSecretsErrorRateHigh:
        // Method: Alert on occurence of errors by looking for a non-zero rate of errors over past 5 minutes
        // Pros:
        // - An app deploy is likely broken if a secret can't be updated by Controller.
        // Caveats:
        // - Probably better to leave app deploy breakages to the app or CD systems monitoring.
        // - Potentially noisy. Controller attempts to unseal 5 times, so if it exceeds on the 4th attempt then all is fine but this alert will trigger.
        // - Usage of an invalid cert.pem with kubeseal will trigger this alert, it would be better to distinguish alerts due to controller or user
        // - 'for' clause not used because we are unlikely to have a sustained rate of errors unless there is a LOT of secret churn in cluster.
        // Rob Ewaschuk - My Philosophy on Alerting: https://docs.google.com/document/d/199PqyG3UsyXlwieHaqbGiWVa8eMWi8zzAn0YfcApr8Q/edit
        {
          alert: 'SealedSecretsUnsealErrorHigh',
          expr: |||
            sum by (reason, namespace) (rate(sealed_secrets_controller_unseal_errors_total{}[5m])) > 0
          ||| % $._config,
          // 'for': '5m', // Not used, see caveats above.
          labels: {
            severity: 'warning',
          },
          annotations: {
            summary: 'Sealed Secrets Unseal Error High',
            description: 'High number of errors during unsealing Sealed Secrets in {{ $labels.namespace }} namespace.',
            runbook_url: 'https://github.com/bitnami-labs/sealed-secrets',
          },
        },
      ],
    }],
  },
}
