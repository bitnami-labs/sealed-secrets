// Minimal required deployment for a functional controller.

local namespace = 'kube-system';

{
  kube:: (import 'vendor_jsonnet/kube-libsonnet/kube.libsonnet') {
    // v1beta2 deprecated in k8s 1.16. v1 can be used since 1.9. We currently officially support only >= 1.13
    // TODO(mkm): remove this override once https://github.com/bitnami-labs/kube-libsonnet/pull/23 lands
    // and we upgrade kube-libsonnet
    Deployment(name): super.Deployment(name) {
      apiVersion: 'apps/v1',
    },
  },
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
