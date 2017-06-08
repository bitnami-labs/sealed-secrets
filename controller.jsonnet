local k = import "ksonnet.beta.1/k.libsonnet";
local util = import "ksonnet.beta.1/util.libsonnet";

local objectMeta = k.core.v1.objectMeta;
local deployment = k.apps.v1beta1.deployment;
local container = k.core.v1.container;
local serviceAccount = k.core.v1.serviceAccount;

local clusterRole(name, rules) = {
  apiVersion: "rbac.authorization.k8s.io/v1beta1",
  kind: "ClusterRole",
  metadata: objectMeta.name(name),
  rules: rules,
};

local role(name, namespace="default", rules) = {
  apiVersion: "rbac.authorization.k8s.io/v1beta1",
  kind: "Role",
  metadata: objectMeta.name(name) + objectMeta.namespace(namespace),
  rules: rules,
};

// eg: "apps/v1beta1" -> "apps"
local apiGroupFromGV(gv) = (
  local group = std.splitLimit(gv, "/", 1)[0];
  if group == "v1" then "" else group
);

local crossGroupRef(target) = {
  kind: target.kind,
  apiGroup: apiGroupFromGV(target.apiVersion),
  name: target.metadata.name,
};

local clusterRoleBinding(name, role, subjects) = {
  apiVersion: "rbac.authorization.k8s.io/v1beta1",
  kind: "ClusterRoleBinding",
  metadata: objectMeta.name(name),
  subjects: [
    crossGroupRef(s) +
      (if std.objectHas(s.metadata, "namespace") then {namespace: s.metadata.namespace}
       else {})
    for s in subjects],
  roleRef: crossGroupRef(role),
};

local roleBinding(name, namespace="default", role, subjects) = {
  apiVersion: "rbac.authorization.k8s.io/v1beta1",
  kind: "RoleBinding",
  metadata: objectMeta.name(name) + objectMeta.namespace(namespace),
  subjects: [
    crossGroupRef(s) +
      (if std.objectHas(s.metadata, "namespace") then {namespace: s.metadata.namespace}
       else {})
    for s in subjects],
  roleRef: crossGroupRef(role),
};

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

local tpr = {
  apiVersion: "extensions/v1beta1",
  kind: "ThirdPartyResource",
  metadata: objectMeta.name("sealed-secret.ksonnet.io"),
  versions: [{name: "v1alpha1"}],
  description: "A sealed (encrypted) Secret",
};

local controllerAccount =
  serviceAccount.default("sealed-secrets-controller", namespace);

local unsealerRole = clusterRole("secrets-unsealer", [
  {
    apiGroups: ["ksonnet.io"],
    resources: ["sealedsecrets"],
    verbs: ["get", "list", "watch"],
  },
  {
    apiGroups: [""],
    resources: ["secrets"],
    verbs: ["create", "update", "delete"],  // don't need get
  },
]);

local sealKeyRole = role("sealed-secrets-key-admin", namespace, [
  {
    apiGroups: [""],
    resources: ["secrets"],
    resourceName: ["sealed-secrets-key"],
    verbs: ["get"],
  },
  {
    apiGroups: [""],
    resources: ["secrets"],
    // Can't limit create by resourceName, because there's no resource yet
    verbs: ["create"],
  },
]);

local binding = clusterRoleBinding("sealed-secrets-controller", unsealerRole, [controllerAccount]);
local binding = roleBinding("sealed-secrets-controller", namespace, sealKeyRole, [controllerAccount]);

local controllerDeployment =
  deployment.default("sealed-secrets-controller", controllerContainer, namespace) +
  deployment.mixin.podSpec.serviceAccountName(controllerAccount.metadata.name) +
  {spec+: {template+: {metadata: {labels: labels}}}};

{
  tpr: util.prune(tpr),
  controller: util.prune(controllerDeployment),
  account: util.prune(controllerAccount),
  unsealerRole: unsealerRole,
  unsealKeyRole: sealKeyRole,
  binding: binding,
}
