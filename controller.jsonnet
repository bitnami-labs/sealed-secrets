// This is the recommended cluster deployment of sealed-secrets.
// See controller-norbac.jsonnet for the bare minimum functionality.

local k = import "ksonnet.beta.1/k.libsonnet";
local controller = import "controller-norbac.jsonnet";

local objectMeta = k.core.v1.objectMeta;
local deployment = k.apps.v1beta1.deployment;
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

local namespace = controller.namespace;

local controllerAccount =
  serviceAccount.default("sealed-secrets-controller", namespace);

local unsealerRole = clusterRole("secrets-unsealer", [
  {
    apiGroups: ["bitnami.com"],
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
    resourceNames: ["sealed-secrets-key"],
    verbs: ["get"],
  },
  {
    apiGroups: [""],
    resources: ["secrets"],
    // Can't limit create by resourceName, because there's no resource yet
    verbs: ["create"],
  },
]);

local unsealerBinding = clusterRoleBinding("sealed-secrets-controller", unsealerRole, [controllerAccount]);
local unsealKeyBinding = roleBinding("sealed-secrets-controller", namespace, sealKeyRole, [controllerAccount]);

controller + {
  controller+: deployment.mixin.podSpec.serviceAccountName(
    controllerAccount.metadata.name),
  account: k.util.prune(controllerAccount),
  unsealerRole: unsealerRole,
  unsealKeyRole: sealKeyRole,
  unsealerBinding: unsealerBinding,
  unsealKeyBinding: unsealKeyBinding,
}
