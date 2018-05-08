// CustomResourceDefinition for SealedSecrets. For K8s >= 1.7
local kube = import "kube.libsonnet";

{
  crd: kube.CustomResourceDefinition("bitnami.com", "v1alpha1", "SealedSecret"),
}
