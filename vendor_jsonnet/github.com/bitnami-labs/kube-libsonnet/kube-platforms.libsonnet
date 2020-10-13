// Extend kube.libsonnet for platform specific CRDs, drop-in usage as:
//
// local kube = import "kube-platforms.jsonnet";
// {
//    my_deploy: kube.Deployment(...) { ... }
//    my_gke_cert: kube.gke.ManagedCertificate(...) { ... }
// }
(import "kube.libsonnet") {
  gke:: {
    ManagedCertificate(name): $._Object("networking.gke.io/v1beta1", "ManagedCertificate", name) {
      spec: {
        domains: error "spec.domains array is required",
      },
      assert std.length(self.spec.domains) > 0 : "ManagedCertificate '%s' spec.domains array must not be empty" % self.metadata.name,
    },

    BackendConfig(name): $._Object("cloud.google.com/v1beta1", "BackendConfig", name) {
      spec: {},
    },
  },
}
