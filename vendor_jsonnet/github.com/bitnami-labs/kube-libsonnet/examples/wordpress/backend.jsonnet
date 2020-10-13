local kube = import "../../kube.libsonnet";

local labels = {
  tier: "backend",
};

{
  backend: {
    secret: kube.Secret("mariadb") {
      metadata+: {
        labels+: labels,
      },
      data_+: {
        "database_name": "webserver_db",
        "database_user": "webserver_user",
        "database_password": "webserver_db_password",
        "replication_user": "replica_user",
        "replication_password": "replica_password",
        "root_user": "root_user",
        "root_password": "root_password"
    }},

    master: {
      local masterLabels = labels + {
        component: "master",
      },
      statefulset: kube.StatefulSet("mariadb-master") {
        metadata+: {
          labels+: masterLabels,
        },
        spec+: {
          template+: {
            spec+: {
              securityContext: {
                runAsUser: 1001,
                fsGroup: 1001,
              },
              containers_+: {
                default: kube.Container("mariadb") {
                  image: "bitnami/mariadb",
                  ports_+: { mysql: { containerPort: 3306 } },
                  env_+: {
                    MARIADB_REPLICATION_MODE: "master",
                    MARIADB_REPLICATION_USER: kube.SecretKeyRef($.backend.secret, "replication_user"),
                    MARIADB_REPLICATION_PASSWORD: kube.SecretKeyRef($.backend.secret, "replication_password"),
                    MARIADB_ROOT_USER: kube.SecretKeyRef($.backend.secret, "root_user"),
                    MARIADB_ROOT_PASSWORD: kube.SecretKeyRef($.backend.secret, "root_password"),
                    MARIADB_USER: kube.SecretKeyRef($.backend.secret, "database_user"),
                    MARIADB_DATABASE: kube.SecretKeyRef($.backend.secret, "database_name"),
                    MARIADB_PASSWORD: kube.SecretKeyRef($.backend.secret, "database_password"),
                  },
                  livenessProbe: {
                    initialDelaySeconds: 40,
                    exec: {
                      command: [
                        "sh",
                        "-c",
                        "exec mysqladmin status -u$MARIADB_ROOT_USER -p$MARIADB_ROOT_PASSWORD",
                  ]}},
                  readinessProbe: self.livenessProbe {
                    initialDelaySeconds: 30,
                  },
                  volumeMounts_+: {
                    "mariadb-data": {
                      "mountPath": "/bitnami/mariadb",
                }}},
                metrics: kube.Container("metrics") {
                  image: "prom/mysqld-exporter:v0.10.0",
                  command: [
                    "sh",
                    "-c",
                    "DATA_SOURCE_NAME=\"$MARIADB_ROOT_USER:$MARIADB_ROOT_PASSWORD@(localhost:3306)/\" exec /bin/mysqld_exporter",
                  ],
                  ports_+: { metrics: { containerPort: 9104 } },
                  env_+: {
                    MARIADB_ROOT_USER: kube.SecretKeyRef($.backend.secret, "root_user"),
                    MARIADB_ROOT_PASSWORD: kube.SecretKeyRef($.backend.secret, "root_password"),
                  },
                  livenessProbe: {
                    initialDelaySeconds: 15,
                    timeoutSeconds: 1,
                    httpGet: {
                      path: "/metrics",
                      port: 9104,
                  }},
                  readinessProbe: self.livenessProbe {
                    initialDelaySeconds: 5,
                    timeoutSeconds: 1,
          }}}}},
          volumeClaimTemplates_+: {
            "mariadb-data": {
              storage: "10Gi",
              metadata+: {
                labels+: masterLabels,
      }}}}},
      service: kube.Service("mariadb-master") {
        metadata+: {
          labels+: masterLabels,
          annotations+: {
            "prometheus.io/scrape": "true",
            "prometheus.io/port": "9104",
        }},
        target_pod: $.backend.master.statefulset.spec.template,
        spec+: {
          ports: [
            {
              name: "mariadb",
              port: 3306,
              targetPort: $.backend.master.statefulset.spec.template.spec.containers[0].ports[0].containerPort,
            },
            {
              name: "metrics",
              port: 9104,
              targetPort: $.backend.master.statefulset.spec.template.spec.containers[1].ports[0].containerPort,
    }]}}},

    slave: {
      local slaveLabels = labels + {
        component: "slave",
      },
      statefulset: kube.StatefulSet("mariadb-slave") {
        metadata+: {
          labels+: slaveLabels,
        },
        spec+: {
          template+: {
            spec+: {
              securityContext: {
                runAsUser: 1001,
                fsGroup: 1001,
              },
              containers_+: {
                default: kube.Container("mariadb") {
                  image: "bitnami/mariadb",
                  ports_+: { mysql: { containerPort: 3306 } },
                  env_+: {
                    MARIADB_REPLICATION_MODE: "slave",
                    MARIADB_REPLICATION_USER: kube.SecretKeyRef($.backend.secret, "replication_user"),
                    MARIADB_REPLICATION_PASSWORD: kube.SecretKeyRef($.backend.secret, "replication_password"),
                    MARIADB_MASTER_HOST: $.backend.master.service.metadata.name,
                    MARIADB_MASTER_ROOT_USER: kube.SecretKeyRef($.backend.secret, "root_user"),
                    MARIADB_MASTER_ROOT_PASSWORD: kube.SecretKeyRef($.backend.secret, "root_password"),
                  },
                  livenessProbe: {
                    initialDelaySeconds: 40,
                    exec: {
                      command: [
                        "sh",
                        "-c",
                        "exec mysqladmin status -u$MARIADB_MASTER_ROOT_USER -p$MARIADB_MASTER_ROOT_PASSWORD",
                  ]}},
                  readinessProbe: self.livenessProbe {
                    initialDelaySeconds: 30,
                  },
                  volumeMounts_+: {
                    "mariadb-data": {
                      "mountPath": "/bitnami/mariadb",
                }}},
                metrics: kube.Container("metrics") {
                  image: "prom/mysqld-exporter:v0.10.0",
                  command: [
                    "sh",
                    "-c",
                    "DATA_SOURCE_NAME=\"$MARIADB_MASTER_ROOT_USER:$MARIADB_MASTER_ROOT_PASSWORD@(localhost:3306)/\" exec /bin/mysqld_exporter",
                  ],
                  ports_+: { metrics: { containerPort: 9104 } },
                  env_+: {
                    MARIADB_MASTER_ROOT_USER: kube.SecretKeyRef($.backend.secret, "root_user"),
                    MARIADB_MASTER_ROOT_PASSWORD: kube.SecretKeyRef($.backend.secret, "root_password"),
                  },
                  livenessProbe: {
                    initialDelaySeconds: 15,
                    timeoutSeconds: 1,
                    httpGet: {
                      path: "/metrics",
                      port: 9104,
                  }},
                  readinessProbe: self.livenessProbe {
                    initialDelaySeconds: 5,
                    timeoutSeconds: 5,
          }}}}},
          volumeClaimTemplates_+: {
            "mariadb-data": {
              storage: "10Gi",
              metadata+: {
                labels+: slaveLabels,
      }}}}},
      service: kube.Service("mariadb-slave") {
        metadata+: {
          labels+: slaveLabels,
          annotations+: {
            "prometheus.io/scrape": "true",
            "prometheus.io/port": "9104",
        }},
        target_pod: $.backend.slave.statefulset.spec.template,
        spec+: {
          ports: [
            {
              name: "mariadb",
              port: 3306,
              targetPort: $.backend.slave.statefulset.spec.template.spec.containers[0].ports[0].containerPort,
            },
            {
              name: "metrics",
              port: 9104,
              targetPort: $.backend.slave.statefulset.spec.template.spec.containers[1].ports[0].containerPort,
}]}}}}}
