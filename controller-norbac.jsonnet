// Minimal required deployment for a functional controller.
local kube = import "kube.libsonnet";

local trim = function(str) (
  if std.startsWith(str, " ") || std.startsWith(str, "\n") then
  trim(std.substr(str, 1, std.length(str) - 1))
  else if std.endsWith(str, " ") || std.endsWith(str, "\n") then
  trim(std.substr(str, 0, std.length(str) - 1))
  else
    str
);

local namespace = "kube-system";
local controllerImage = std.extVar("CONTROLLER_IMAGE");

// This is a bit odd: Downgrade to apps/v1beta1 so we can continue
// to support k8s v1.6.
// TODO: re-evaluate sealed-secrets support timeline and/or
// kube.libsonnet versioned API support.
local v1beta1_Deployment(name) = kube.Deployment(name) {
  assert std.assertEqual(super.apiVersion, "apps/v1beta2"),
  apiVersion: "apps/v1beta1",
};

{
  namespace:: {metadata+: {namespace: namespace}},

  service: kube.Service("sealed-secrets-controller") + $.namespace {
    target_pod: $.controller.spec.template,
  },

  controller: v1beta1_Deployment("sealed-secrets-controller") + $.namespace {
    spec+: {
      template+: {
        spec+: {
          containers_+: {
            controller: kube.Container("sealed-secrets-controller") {
              image: controllerImage,
              command: ["controller"],
              readinessProbe: {
                httpGet: {path: "/healthz", port: "http"},
              },
              livenessProbe: self.readinessProbe,
              ports_+: {
                http: {containerPort: 8080},
              },
              securityContext+: {
                readOnlyRootFilesystem: true,
                runAsNonRoot: true,
                runAsUser: 1001,
              },
            },
          },
        },
      },
    },
  },
}
