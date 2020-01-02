// Minimal required deployment for a functional controller.

local namespace = 'kube-system';

{
  kube:: (import 'vendor_jsonnet/kube-libsonnet/kube.libsonnet'),
  local kube = self.kube,

  controllerImage:: std.extVar('CONTROLLER_IMAGE'),
  imagePullPolicy:: std.extVar('IMAGE_PULL_POLICY'),

  crd: kube.CustomResourceDefinition('bitnami.com', 'v1alpha1', 'SealedSecret'),

  namespace:: { metadata+: { namespace: namespace } },

  service: kube.Service('sealed-secrets-controller') + $.namespace {
    target_pod: $.controller.spec.template,
  },

  controller: kube.Deployment('sealed-secrets-controller') + $.namespace {
    spec+: {
      template+: {
        spec+: {
          securityContext+: {
            fsGroup: 65534,
          },
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
