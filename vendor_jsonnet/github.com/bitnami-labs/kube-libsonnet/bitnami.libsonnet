// Generic stuff is in kube.libsonnet - this file contains
// bitnami-specific conventions.

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

  Ingress(name, class=null): kube.Ingress(name) {
    local ing = self,

    host:: error "host required",
    target_svc:: error "target_svc required",
    // Default to single-service - override if you want something else.
    paths:: [{ path: "/", backend: ing.target_svc.name_port }],
    secretName:: "%s-cert" % [ing.metadata.name],

    // cert_provider can either be:
    // - "cm-dns": cert-manager using route53 for ACME dns-01 challenge (default)
    // - "cm-http": cert-manager using ACME http, requires public ingress
    cert_provider:: $.CertManager.default_ingress_provider,

    metadata+: $.CertManager.IngressMeta[ing.cert_provider] {
      annotations+: {
        // Add ingress class iff specified
        [if class != null then "kubernetes.io/ingress.class" else null]: class,
      },
    },
    spec+: {
      tls: [
        {
          hosts: std.set([r.host for r in ing.spec.rules]),
          secretName: ing.secretName,
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
  CertManager:: {
    // Deployed cluster issuers' names:
    cluster_issuers:: {
      acme_dns:: "letsencrypt-prod-dns",
      acme_http:: "letsencrypt-prod-http",
      in_cluster:: "in-cluster-issuer",
    },

    default_ingress_provider:: "cm-dns",
    IngressMeta:: {
      "cm-dns":: {
        annotations+: {
          "cert-manager.io/cluster-issuer": $.CertManager.cluster_issuers.acme_dns,
        },
      },
      "cm-http":: {
        annotations+: {
          "cert-manager.io/cluster-issuer": $.CertManager.cluster_issuers.acme_http,
        },
      },
    },


    // CertManager ClusterIssuer object
    ClusterIssuer(name):: kube._Object("cert-manager.io/v1alpha2", "ClusterIssuer", name),

    // CertManager Certificate object
    Certificate(name):: kube._Object("cert-manager.io/v1alpha2", "Certificate", name) {
      assert std.objectHas(self.metadata, "namespace") : "Certificate('%s') must set metadata.namespace" % self.metadata.name,
    },

    InCluster:: {
      // Broadest usage is ["any"], limit to mTLS usage:
      default_usages:: ["digital signature", "key encipherment"],
      // Ref to our in-cluster ClusterIssuer
      cluster_issuer:: $.CertManager.ClusterIssuer($.CertManager.cluster_issuers.in_cluster) {
        spec+: {
          selfSigned: {},
        },
      },
      // Use as:
      //   my_cert: kube.CertManager.InCluster.Certificate("my-tls-cert", "my-namespace")
      // to get a Kubernetes TLS secret named "my-tls-cert" in "my-namespace"
      Certificate(name, namespace):: $.CertManager.Certificate(name) {
        metadata+: { namespace: namespace },
        spec+: {
          secretName: name,
          issuerRef: kube.CrossVersionObjectReference($.CertManager.InCluster.cluster_issuer) {
            // issuerRef doesn't have the apiVersion field
            apiVersion:: null,
          },
          commonName: name,
          dnsNames: [
            name,
            "%s.%s" % [name, namespace],
            "%s.%s.svc" % [name, namespace],
          ],
          usages: $.CertManager.InCluster.default_usages,
        },
      },
    },
  },
}
