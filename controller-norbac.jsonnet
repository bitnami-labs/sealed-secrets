// Minimal required deployment for a functional controller.

local namespace = 'kube-system';

{
  kube:: (import 'vendor_jsonnet/kube-libsonnet/kube.libsonnet'),
  local kube = self.kube,

  controllerImage:: std.extVar('CONTROLLER_IMAGE'),
  imagePullPolicy:: std.extVar('IMAGE_PULL_POLICY'),

  crd: kube.CustomResourceDefinition('bitnami.com', 'v1alpha1', 'SealedSecret') {
    spec+: {
      subresources: {
        status: {},
      }
    },
  },

  namespace:: { metadata+: { namespace: namespace } },
  managedBy:: 'jsonnet',
  labels:: {
    metadata+: {
      labels+: {
        'app.kubernetes.io/name': 'kubeseal',
        'app.kubernetes.io/version': std.splitLimit($.controllerImage, ':', 1)[1],
        'app.kubernetes.io/part-of': 'kubeseal',
        'app.kubernetes.io/managed-by': $.managedBy,
      },
    },
  },

  service: kube.Service('sealed-secrets-controller') + $.namespace + $.labels {
    target_pod: $.controller.spec.template,
  },

  controller: kube.Deployment('sealed-secrets-controller') + $.namespace + $.labels {
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
