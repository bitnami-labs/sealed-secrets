// ThirdPartyResource for SealedSecrets. For K8s <= 1.7
local k = import "ksonnet.beta.1/k.libsonnet";

local objectMeta = k.core.v1.objectMeta;

local tpr = {
  apiVersion: "extensions/v1beta1",
  kind: "ThirdPartyResource",
  metadata: objectMeta.name("sealed-secret.bitnami.com"),
  versions: [{name: "v1alpha1"}],
  description: "A sealed (encrypted) Secret",
};

{
  tpr: tpr,
}
