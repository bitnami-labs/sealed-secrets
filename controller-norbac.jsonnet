// Minimal required deployment for a functional TPR + controller.
local k = import "ksonnet.beta.1/k.libsonnet";

local objectMeta = k.core.v1.objectMeta;
local deployment = k.apps.v1beta1.deployment;
local container = k.core.v1.container;
local probe = k.core.v1.probe;
local service = k.core.v1.service;
local servicePort = k.core.v1.servicePort;

local trim = function(str) (
  if std.startsWith(str, " ") || std.startsWith(str, "\n") then
  trim(std.substr(str, 1, std.length(str) - 1))
  else if std.endsWith(str, " ") || std.endsWith(str, "\n") then
  trim(std.substr(str, 0, std.length(str) - 1))
  else
    str
);

local namespace = "kube-system";

local controllerImage = trim(importstr "controller.image");
local controllerPort = 8080;

local controllerProbe =
  probe.default() +
  probe.mixin.httpGet.path("/healthz") +
  probe.mixin.httpGet.port(controllerPort);

local controllerContainer =
  container.default("sealed-secrets-controller", controllerImage) +
  container.command(["controller"]) +
  container.livenessProbe(controllerProbe) +
  container.readinessProbe(controllerProbe) +
  container.mixin.securityContext.readOnlyRootFilesystem(true) +
  container.mixin.securityContext.runAsUser(1001) +
  container.helpers.namedPort("http", controllerPort);

local labels = {name: "sealed-secrets-controller"};

local tpr = {
  apiVersion: "extensions/v1beta1",
  kind: "ThirdPartyResource",
  metadata: objectMeta.name("sealed-secret.bitnami.com"),
  versions: [{name: "v1alpha1"}],
  description: "A sealed (encrypted) Secret",
};

local controllerDeployment =
  deployment.default("sealed-secrets-controller", controllerContainer, namespace) +
  {spec+: {template+: {metadata: {labels: labels}}}};

local controllerSvc =
  service.default("sealed-secrets-controller", namespace) +
  service.spec(k.core.v1.serviceSpec.default()) +
  service.mixin.spec.selector(labels) +
  service.mixin.spec.ports([servicePort.default(controllerPort)]);

{
  namespace:: namespace,
  tpr: k.util.prune(tpr),
  controller: k.util.prune(controllerDeployment),
  service: k.util.prune(controllerSvc),
}
