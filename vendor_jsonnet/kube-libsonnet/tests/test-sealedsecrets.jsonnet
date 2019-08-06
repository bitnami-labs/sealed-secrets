local kube = import "../kube.libsonnet";
local stack = {
  sealedsecret: kube.SealedSecret("foo") {
    spec+: {
      data: "dGVzdAo=",
    },
  },
};

kube.List() {
  items_+: stack,
}
