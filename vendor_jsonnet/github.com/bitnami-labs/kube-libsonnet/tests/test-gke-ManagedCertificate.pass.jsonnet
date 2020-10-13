local kube = import "../kube-platforms.libsonnet";
local stack = {
  foocert: kube.gke.ManagedCertificate("foo") {
    spec+: {
      domains: ["foo.example.com"],
    },
  },
};

kube.List() {
  items_+: stack,
}
