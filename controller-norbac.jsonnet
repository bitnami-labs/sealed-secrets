// Minimal required deployment for a functional controller.
local kube = import 'kube.libsonnet';

local namespace = 'kube-system';

// This is a bit odd: Downgrade to apps/v1beta1 so we can continue
// to support k8s v1.6.
// TODO: re-evaluate sealed-secrets support timeline and/or
// kube.libsonnet versioned API support.
local v1beta1_Deployment(name) = kube.Deployment(name) {
  assert std.assertEqual(super.apiVersion, 'apps/v1beta2'),
  apiVersion: 'apps/v1beta1',
};

{
  controllerImage:: std.extVar('CONTROLLER_IMAGE'),
  imagePullPolicy:: std.extVar('IMAGE_PULL_POLICY'),

  crd: kube.CustomResourceDefinition('bitnami.com', 'v1alpha1', 'SealedSecret'),

  namespace:: { metadata+: { namespace: namespace } },

  service: kube.Service('sealed-secrets-controller') + $.namespace {
    target_pod: $.controller.spec.template,
  },

  controller: v1beta1_Deployment('sealed-secrets-controller') + $.namespace {
    spec+: {
      template+: {
        spec+: {
          containers_+: {
            controller: kube.Container('sealed-secrets-controller') {
              image: $.controllerImage,
              imagePullPolicy: $.imagePullPolicy,
              command: ['controller'],
              readinessProbe: {
                httpGet: { path: '/healthz', port: 'http' },
              },
              livenessProbe: self.readinessProbe,
              ports_+: {
                http: { containerPort: 8080 },
              },
              securityContext+: {
                readOnlyRootFilesystem: true,
                runAsNonRoot: true,
                runAsUser: 1001,
              },
              volumeMounts_+: {
                tmp: {
                  mountPath: '/tmp',
                },
              },
            },
          },
          volumes_+: {
            tmp: {
              emptyDir: {},
            },
          },
        },
      },
    },
  },
}
