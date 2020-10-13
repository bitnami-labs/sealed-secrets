local kube = import "../kube.libsonnet";
local utils = import "../utils.libsonnet";

local an_obj = kube._Object("v1", "Gentle", "foo");
local a_pod = kube.Pod("foo") {
  metadata+: { labels+: { foo: "bar", bar: "qxx" } },
  spec+: {
    containers_+: {
      foo: kube.Container("foo") {
        image: "nginx",
        ports_: {
          http: { containerPort: 8080 },
          https: { containerPort: 8443 },
          udp: { containerPort: 5353, protocol: "UDP" },
        },
      },
    },
  },
};
local a_deploy = kube.Deployment("foo") {
  spec+: { template+: { metadata+: a_pod.metadata, spec+: a_pod.spec } },
};
// Basic unittesting for methods that are not exercised by the other e2e-ish tests
// kube.libsonnet
std.assertEqual(kube.objectValues({ a: 1, b: 2 }), [1, 2]) &&
std.assertEqual(kube.objectItems({ a: 1, b: 2 }), [["a", 1], ["b", 2]]) &&
std.assertEqual(kube.hyphenate("foo_bar_baz"), ("foo-bar-baz")) &&
std.assertEqual(kube.mapToNamedList({ foo: { a: "b" } }), [{ name: "foo", a: "b" }]) &&
std.assertEqual(kube.filterMapByFields({ a: 1, b: 2, c: 3 }, ["a", "c", "d"]), { a: 1, c: 3 }) &&
std.assertEqual(kube.parseOctal("755"), 493) &&
std.assertEqual(kube.siToNum("42G"), 42 * 1e9) &&
std.assertEqual(kube.siToNum("42Gi"), 42 * std.pow(2, 30)) &&
std.assertEqual(kube.toUpper("ForTy 2"), "FORTY 2") &&
std.assertEqual(kube.toLower("ForTy 2"), "forty 2") &&
std.assertEqual(an_obj, {
  apiVersion: "v1",
  kind: "Gentle",
  metadata: { name: "foo", labels: { name: "foo" }, annotations: {} },
}) &&
std.assertEqual(
  [kube.podRef(a_deploy).spec.ports("TCP"), kube.podRef(a_deploy).spec.ports("UDP")],
  [[8080, 8443], [5353]]
) &&
std.assertEqual(
  // latest kubecfg produces stable output from maps hashes, so below shouldn't be flaky
  kube.podsPorts([a_deploy]),
  [
    { port: 8080, protocol: "TCP" },
    { port: 8443, protocol: "TCP" },
    { port: 5353, protocol: "UDP" },
  ]
) &&
std.assertEqual(
  kube.podLabelsSelector(a_deploy),
  { podSelector: { matchLabels: { name: "foo", foo: "bar", bar: "qxx" } } }
) &&
// utils.libsonnet
std.assertEqual(
  [utils.path_join("foo", "bar"), utils.path_join("foo/", "bar")],
  ["foo/bar", "foo/bar"]
) &&
std.assertEqual(
  utils.trimUrl("http://example.com/foo/"),
  "http://example.com/foo"
) &&
std.assertEqual(
  std.parseJson(utils.toJson('{ "foo": "bar\nqqq" }')),
  '{ "foo": "bar\nqqq" }',
) &&
std.assertEqual(
  utils.parentDomain("foo.example.com"),
  "example.com"
) &&
std.assertEqual(
  utils.parentDomain("foo.example.com"),
  "example.com"
) &&
std.assertEqual(
  std.uniq([
    x.podAffinityTerm.labelSelector
    for x in utils.weakNodeDiversity({ foo: "bar" }).podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution
  ]),
  [{ foo: "bar" }]
) &&
std.assertEqual(
  (utils.HashedConfigMap("hashed-cm")) { data+: { foo: "bar" } }.metadata.name,
  "hashed-cm-94232c5",
) &&
std.assertEqual(
  (
    (utils.HashedConfigMap("hashed-cm")) {
      data+: { foo: "bar" },
    }.metadata.name
    ==
    (utils.HashedConfigMap("hashed-cm")) {
      data+: { foo: "baz" },
    }.metadata.name
  ),
  false,
) &&
std.assertEqual(
  (utils.HashedSecret("hashed-secret")) { data+: { foo: std.base64("bar") } }.metadata.name,
  "hashed-secret-16f81db",
) &&
std.assertEqual(
  (
    (utils.HashedSecret("hashed-secret")) {
      data+: { foo: std.base64("bar") },
    }.metadata.name
    ==
    (utils.HashedSecret("hashed-secret")) {
      data+: { foo: std.base64("baz") },
    }.metadata.name
  ),
  false,
) &&
true
