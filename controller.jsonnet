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

local controllerImage = trim(importstr "controller.image");

local controllerContainer =
  container.default("sealed-secrets-controller", controllerImage) +
  container.imagePullPolicy("IfNotPresent");

local labels = {name: "sealed-secrets-controller"};

local controllerDeployment =
  deployment.default("sealed-secrets-controller", controllerContainer) +
  {spec+: {template+: {metadata: {labels: labels}}}};

//util.prune(controllerDeployment)

local kube = import "kube.libsonnet";

{
  deploy: kube.Deployment("sealed-secrets-controller") {
    spec+: {
      template+: {
        spec+: {
          containers_+: {
            controller: kube.Container("controller") {
              image: controllerImage,
              imagePullPolicy: "IfNotPresent",
              command: ["controller"],
              args_+: {
                logtostderr: "true",
                v: 9,
              }}}}}}},
  // tpr: kube.ThirdPartyResource("sealed-secret.ksonnet.io") {
  //   versions_: ["v1alpha1"],
  //   description: "A sealed (encrypted) Secret.",
  // },
}
