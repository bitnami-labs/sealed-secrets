// ThirdPartyResource for SealedSecrets. For K8s <= 1.7
local kube = import "kube.libsonnet";

{
  tpr: kube.ThirdPartyResource("sealed-secret.bitnami.com") {
    versions_: ["v1alpha1"],
    description: "A sealed (encrypted) Secret",
  },
}
