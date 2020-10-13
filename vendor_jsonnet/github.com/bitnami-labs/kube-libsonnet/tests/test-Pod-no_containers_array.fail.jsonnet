local kube = import "../kube.libsonnet";
local simple_validate = (import "test-simple-validate.pass.jsonnet").items_;
simple_validate {
  pod+: {
    spec+: {
      containers: [],
    },
  },
}
