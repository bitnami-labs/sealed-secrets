local kube = import "../kube.libsonnet";
local simple_validate = (import "test-simple-validate.pass.jsonnet").items_;
simple_validate {
  pod+: {
    metadata+: {
      spec+: {
        containers_+: {
          foo_cont+: {
            env_+: {
              my_secret: kube.SecretKeyRef($.secret, "sec_key_nopes"),
            },
          },
        },
      },
    },
  },
}
