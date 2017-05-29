local kube = import "kube.libsonnet";

{
  mysecret: kube.Secret("mysecret") {
    data_: {
      foo: "bar",
    },
  },
}
