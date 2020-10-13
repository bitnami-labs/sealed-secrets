local kube = import "../kube.libsonnet";
local simple_validate = (import "test-simple-validate.pass.jsonnet").items_;
simple_validate {
  deploy_pdb+: {
    spec+: {
      minAvailable:: null,
    },
  },
}
