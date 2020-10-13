local kube = import "../kube.libsonnet";
local stack = {
  sealedsecret: kube.SealedSecret("foo") {
    spec+: {
      encryptedData: std.parseJson(importstr "test-SealedSecret.pass.json"),
    },
  },
};

kube.List() {
  items_+: stack,
}
