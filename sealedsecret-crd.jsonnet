// CustomResourceDefinition for SealedSecrets. For K8s >= 1.7
local k = import "ksonnet.beta.1/k.libsonnet";

local objectMeta = k.core.v1.objectMeta;

local crd = {
  apiVersion: "apiextensions.k8s.io/v1beta1",
  kind: "CustomResourceDefinition",
  metadata: objectMeta.name($.spec.names.plural + "." + $.spec.group),
  spec: {
    scope: "Namespaced",
    group: "bitnami.com",
    version: "v1alpha1",
    names: {
      kind: "SealedSecret",
      singular: "sealedsecret",
      plural: self.singular + "s",
    },
  },
};

{
  crd: crd,
}
