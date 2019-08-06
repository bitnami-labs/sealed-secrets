local kube = import "../kube.libsonnet";
local stack = {
  sealedsecret: kube.SealedSecret("foo") {
    spec+: {
      datalines_: importstr "test-sealedsecrets-datalines.txt",
    },
  },
};

kube.List() {
  items_+: stack,
}
