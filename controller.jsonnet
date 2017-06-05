local k = import "ksonnet.beta.1/k.libsonnet";
local util = import "ksonnet.beta.1/util.libsonnet";

local deployment = k.apps.v1beta1.deployment;
local container = k.core.v1.container;

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

local controllerContainer =
  container.default("sealed-secrets-controller", controllerImage) +
  container.command(["controller"]) +
  container.args(["--logtostderr"]);

local labels = {name: "sealed-secrets-controller"};

local controllerDeployment =
  deployment.default("sealed-secrets-controller", controllerContainer, namespace) +
  {spec+: {template+: {metadata: {labels: labels}}}};

util.prune(controllerDeployment)
