local kube = import "../kube.libsonnet";
local stack = {
  sealedsecret: kube.SealedSecret("foo") {
    spec+: {
      bar: std.parseJson(importstr "test-sealedsecrets.json"),
    },
  },
};

kube.List() {
  items_+: stack,
}
