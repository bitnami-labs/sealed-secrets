// Generic stuff is in kube.libsonnet - this file contains
// additional AWS or Bitnami -specific conventions.

local kube = import "kube.libsonnet";

local perCloudSvcAnnotations(cloud, internal, service) = (
  {
    aws: {
      "service.beta.kubernetes.io/aws-load-balancer-connection-draining-enabled": "true",
      "service.beta.kubernetes.io/aws-load-balancer-connection-draining-timeout": std.toString(service.target_pod.spec.terminationGracePeriodSeconds),
      // Use PROXY protocol (nginx supports this too)
      "service.beta.kubernetes.io/aws-load-balancer-proxy-protocol": "*",
      // Does LB do NAT or DSR? (OnlyLocal implies DSR)
      // https://kubernetes.io/docs/tutorials/services/source-ip/
      // NB: Don't enable this without modifying set-real-ip-from above!
      // Not supported on aws in k8s 1.5 - immediate close / serves 503s.
      //"service.beta.kubernetes.io/external-traffic": "OnlyLocal",
    },
    gke: {},
  }[cloud] + if internal then {
    aws: {
      "service.beta.kubernetes.io/aws-load-balancer-internal": "0.0.0.0/0",
    },
    gke: {
      "cloud.google.com/load-balancer-type": "internal",
    },
  }[cloud] else {}
);

local perCloudSvcSpec(cloud) = (
  {
    aws: {},
    // Required to get real src IP address, which also allows proper
    // ingress.kubernetes.io/whitelist-source-range matching
    gke: { externalTrafficPolicy: "Local" },
  }[cloud]
);

{
  ElbService(name, cloud, internal): kube.Service(name) {
    local service = self,

    metadata+: {
      annotations+: perCloudSvcAnnotations(cloud, internal, service),
    },
    spec+: { type: "LoadBalancer" } + perCloudSvcSpec(cloud),
  },

  Ingress(name): kube.Ingress(name) {
    local ing = self,

    host:: error "host required",
    target_svc:: error "target_svc required",
    // Default to single-service - override if you want something else.
    paths:: [{ path: "/", backend: ing.target_svc.name_port }],
    secretName:: "%s-cert" % [ing.metadata.name],
    // cert_provider can either be:
    // - "kcm": uses route53 for ACME dns-01 challenge
    // - "lego": requires public ingress, uses http for ACME http challenge
    cert_provider:: "kcm",

    kcm_metadata:: {
      annotations+: {
        "stable.k8s.psg.io/kcm.provider": "route53",
        "stable.k8s.psg.io/kcm.email": "sre@bitnami.com",
      },
      labels+: {
        "stable.k8s.psg.io/kcm.class": "default",
      },
    },
    kube_lego_metadata:: {
      annotations+: {
        "kubernetes.io/tls-acme": "true",
      },
    },

    metadata+: if ing.cert_provider == "kcm" then ing.kcm_metadata else ing.kube_lego_metadata,
    spec+: {
      tls: [
        {
          hosts: std.set([r.host for r in ing.spec.rules]),
          secretName: ing.secretName,

          assert std.length(self.hosts) <= 1 : "kube-cert-manager only supports one host per secret - make a separate Ingress resource",
        },
      ],

      rules: [
        {
          host: ing.host,
          http: {
            paths: ing.paths,
          },
        },
      ],
    },
  },

  PromScrape(port): {
    local scrape = self,
    prom_path:: "/metrics",

    metadata+: {
      annotations+: {
        "prometheus.io/scrape": "true",
        "prometheus.io/port": std.toString(port),
        "prometheus.io/path": scrape.prom_path,
      },
    },
  },

  PodZoneAntiAffinityAnnotation(pod): {
    podAntiAffinity: {
      preferredDuringSchedulingIgnoredDuringExecution: [
        {
          weight: 50,
          podAffinityTerm: {
            labelSelector: { matchLabels: pod.metadata.labels },
            topologyKey: "failure-domain.beta.kubernetes.io/zone",
          },
        },
        {
          weight: 100,
          podAffinityTerm: {
            labelSelector: { matchLabels: pod.metadata.labels },
            topologyKey: "kubernetes.io/hostname",
          },
        },
      ],
    },
  },
}
