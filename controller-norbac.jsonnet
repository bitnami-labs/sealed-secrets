// Minimal required deployment for a functional controller.

local kubecfg = import 'kubecfg.libsonnet';

local namespace = 'kube-system';

{
  kube:: (import 'vendor_jsonnet/kube-libsonnet/kube.libsonnet'),
  local kube = self.kube + import 'kube-fixes.libsonnet',

  controllerImage:: std.extVar('CONTROLLER_IMAGE'),
  imagePullPolicy:: local ext = std.extVar('IMAGE_PULL_POLICY'); if ext == '' then
    if std.endsWith($.controllerImage, ':latest') then 'Always' else 'IfNotPresent'
  else ext,

  crd: kube.CustomResourceDefinition('bitnami.com', 'v1alpha1', 'SealedSecret') {
    spec+: {
      versions_+: {
        v1alpha1+: {
          served: true,
          storage: true,
          subresources: {
            status: {},
          },
          schema: kubecfg.parseYaml(importstr 'schema-v1alpha1.yaml')[0],
        },
      },
    },
  },

  namespace:: { metadata+: { namespace: namespace } },

  service: kube.Service('sealed-secrets-controller') + $.namespace {
    target_pod: $.controller.spec.template,
  },

  service_metrics: kube.Service('sealed-secrets-controller-metrics') + $.namespace {
    local service = self,
    target_pod: $.controller.spec.template,
    spec: {
      selector: service.target_pod.metadata.labels,
      ports: [
        {
          port: 8081,
          targetPort: 8081,
        },
      ],
      type: "ClusterIP",
    },
  },

  controller: kube.Deployment('sealed-secrets-controller') + $.namespace {
    spec+: {
      template+: {
        spec+: {
          securityContext+: {
            fsGroup: 65534,
            runAsNonRoot: true,
            runAsUser: 1001,
            seccompProfile+: {
              type: 'RuntimeDefault',
            }
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
                metrics: { containerPort: 8081 },
              },
              securityContext+: {
                allowPrivilegeEscalation: false,
                capabilities+: {
                  drop: [ 'ALL' ],
                },
                readOnlyRootFilesystem: true,
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
