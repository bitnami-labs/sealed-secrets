local bitnami = import "../bitnami.libsonnet";
local kube = import "../kube.libsonnet";
local utils = import "../utils.libsonnet";

local stack = {
  namespace:: "foons",
  name:: "foo",

  ns: kube.Namespace($.namespace),

  sa: kube.ServiceAccount($.name + "-sa") {
    metadata+: { namespace: $.namespace },
  },

  role: kube.Role($.name + "-role") {
    metadata+: { namespace: $.namespace },
    rules: [{
      apiGroups: [""],
      resources: ["pods", "secrets", "configmaps", "persistentvolumeclaims"],
      verbs: ["get"],
    }, {
      apiGroups: [""],
      resources: ["pods"],
      verbs: ["patch"],
    }],
  },

  rolebinding: kube.RoleBinding($.name + "-rolebinding") {
    metadata+: { namespace: $.namespace },
    roleRef_: $.role,
    subjects_+: [$.sa],
  },

  config: kube.ConfigMap($.name + "-config") {
    metadata+: { namespace: $.namespace },
    data: {
      foo_key: "bar_val",
    },
  },

  secret: kube.Secret($.name + "-secret") {
    metadata+: { namespace: $.namespace },
    data: {
      sec_key: "c2VjcmV0Cg==",
    },
  },

  // NB: making up an Ingress pointing to $.deploy Pod
  service: kube.Service($.name + "-svc") {
    metadata+: { namespace: $.namespace },
    target_pod: $.deploy.spec.template,
  },

  ingress: bitnami.Ingress($.name + "-ingress") {
    metadata+: { namespace: $.namespace },
    host: "foo.g.dev.bitnami.net",
    target_svc: $.service,
  },

  // NB: just a simple example pod
  pod: kube.Pod($.name + "-pod") {
    metadata+: { namespace: $.namespace },
    spec+: {
      containers_+: {
        foo_cont: kube.Container($.name) {
          image: "nginx:1.12",
          env_+: {
            my_secret: kube.SecretKeyRef($.secret, "sec_key"),
            other_key: null,
          },
          ports_+: {
            http: { containerPort: 80 },
            udp_port: { containerPort: 888, protocol: "UDP" },
          },
          volumeMounts_+: {
            config_vol: { mountPath: "/config" },
          },
        },
      },
      volumes_+: {
        config_vol: kube.ConfigMapVolume($.config),
      },
    },
  },

  // NB: all object below needing to spec a Pod will just
  // use above particular pod manifest just for convenience
  deploy: kube.Deployment($.name + "-deploy") {
    local this = self,
    metadata+: { namespace: $.namespace },
    spec+: {
      template+: {
        spec+: $.pod.spec {
          affinity+: utils.weakNodeDiversity(this.spec.selector),
          serviceAccountName: $.sa.metadata.name,
        },
      },
    },
  },

  deploy_pdb: kube.PodDisruptionBudget($.name + "-deploy-pdb") {
    target_pod: $.deploy.spec.template,
    spec+: {
      minAvailable: 1,
    },
  },

  sts: kube.StatefulSet($.name + "-sts") {
    metadata+: { namespace: $.namespace },
    spec+: {
      template+: {
        spec+: $.pod.spec {
          serviceAccountName: $.sa.metadata.name,
          containers_+: {
            foo_cont+: {
              volumeMounts_+: {
                datadir: { mountPath: "/foo/data" },
              },
            },
          },
        },
      },
      volumeClaimTemplates_+: {
        datadir: kube.PersistentVolumeClaim("datadir") {
          metadata+: { namespace: $.namespace },
          storage: "10Gi",
        },
      },
    },
  },

  ds: kube.DaemonSet($.name + "-ds") {
    metadata+: { namespace: $.namespace },
    spec+: {
      template+: {
        spec: $.pod.spec,
      },
    },
  },

  job: kube.Job($.name + "-job") {
    metadata+: { namespace: $.namespace },
    spec+: {
      template+: {
        spec+: {
          containers_+: {
            foo_cont: kube.Container($.name) {
              image: "busybox",
            },
          },
        },
      },
    },
  },

  cronjob: kube.CronJob($.name + "-cronjob") {
    metadata+: { namespace: $.namespace },
    spec+: {
      jobTemplate+: {
        spec+: {
          template+: {
            spec+: {
              containers_+: {
                foo_cont: kube.Container($.name) {
                  image: "busybox",
                },
              },
            },
          },
        },
      },
      schedule: "0 * * * *",
    },
  },

  // NB: create NSP from $.deploy Pod ref
  nsp_pods: kube.NetworkPolicy($.name + "-nsp-pods") {
    metadata+: { namespace: $.namespace },
    // NB: $.deploy has unique "foo-deploy" label (as well as other
    // podLabelsSelector() arg)
    spec+: kube.podLabelsSelector($.deploy) {
      // NB: making up $.deploy needing to get reached by $job, $.cronjob
      // and nginx-ingress-controller (running in its own NS named "nginx-ingress"
      ingress_: {
        from_jobs_and_ingressctl: {
          from: [
            kube.podLabelsSelector($.job),
            kube.podLabelsSelector($.cronjob),
            { namespaceSelector: { matchLabels: { name: "nginx-ingress" } } },
          ],
          ports: kube.podsPorts([$.deploy]),
        },
      },
      // NB: making up $.deploy needing to connect to $.sts, and
      // "kube-system" NS for DNS services
      egress_: {
        to_sts: {
          to: [
            kube.podLabelsSelector($.sts),
          ],
          ports: kube.podsPorts([$.sts]),
        },
        to_kube_dns: {
          to: [
            { namespaceSelector: { matchLabels: { name: "kube-system" } } },
          ],
          ports: [{ port: 53, protocol: "UDP" }],
        },
      },
    },
  },
  // NB: these VPAs need the VPA CRD added to the cluster, for local k3s testing
  // we add it via the `init-kube` Makefile target using `init-kube.jsonnet`
  vpa: kube.VerticalPodAutoscaler($.name + "-vpa") {
    spec+: {
      targetRef: {
        apiVersion: "apps/v1",
        kind: "Deployment",
        name: "foo-deploy",
      },
    },
  },
  deploy_vpa: kube.createVPAFor($.deploy),
  tls_cert: bitnami.CertManager.InCluster.Certificate("foo-cert", $.namespace),
};

kube.List() {
  items_+: stack,
}
