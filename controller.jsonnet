// This is the recommended cluster deployment of sealed-secrets.
// See controller-norbac.jsonnet for the bare minimum functionality.

local controller = import 'controller-norbac.jsonnet';

controller {
  local kube = self.kube,

  account: kube.ServiceAccount('sealed-secrets-controller') + $.namespace,

  unsealerRole: kube.ClusterRole('secrets-unsealer') {
    rules: [
      {
        apiGroups: ['bitnami.com'],
        resources: ['sealedsecrets'],
        verbs: ['get', 'list', 'watch', 'update'],
      },
      {
        apiGroups: [''],
        resources: ['secrets'],
        verbs: ['get', 'create', 'update', 'delete'],
      },
      {
        apiGroups: [''],
        resources: ['events'],
        verbs: ['create', 'patch'],
      },
    ],
  },

  unsealKeyRole: kube.Role('sealed-secrets-key-admin') + $.namespace {
    rules: [
      {
        apiGroups: [''],
        resources: ['secrets'],
        // Can't limit create by resource name as keys are produced on the fly
        verbs: ['create', 'list'],
      },
    ],
  },

  serviceProxierRole: kube.Role('sealed-secrets-service-proxier') + $.namespace {
    rules: [
      {
        apiGroups: [
          '',
        ],
        resources: [
          'services/proxy',
        ],
        resourceNames: [
          'http:sealed-secrets-controller:',  // kubeseal uses net.JoinSchemeNamePort when crafting proxy subresource URLs
          'sealed-secrets-controller',  // but often services are referred by name only, let's not make it unnecessarily cryptic
        ],
        verbs: [
          'create',  // rotate and validate endpoints expect POST, see https://kubernetes.io/docs/reference/access-authn-authz/authorization/#determine-the-request-verb
          'get',
        ],
      },
    ],
  },

  unsealerBinding: kube.ClusterRoleBinding('sealed-secrets-controller') {
    roleRef_: $.unsealerRole,
    subjects_+: [$.account],
  },

  unsealKeyBinding: kube.RoleBinding('sealed-secrets-controller') + $.namespace {
    roleRef_: $.unsealKeyRole,
    subjects_+: [$.account],
  },

  serviceProxierBinding: kube.RoleBinding('sealed-secrets-service-proxier') + $.namespace {
    roleRef_: $.serviceProxierRole,
    // kube.libsonnet assumes object here have a namespace, but system groups don't
    // thus are not supposed to use the magic "_" here.
    subjects+: [kube.Group('system:authenticated')],
  },

  controller+: {
    spec+: {
      template+: {
        spec+: {
          serviceAccountName: $.account.metadata.name,
        },
      },
    },
  },
}
