local kube = import "../../kube.libsonnet";
local be = import "backend.jsonnet";

local labels = {
  tier: "frontend",
};

{
  frontend: {
    pvc: kube.PersistentVolumeClaim("wordpress") {
      metadata+: {
        labels+: labels,
      },
      storage:: "10Gi",
    },

    configmap: kube.ConfigMap("wordpress") {
      metadata+: {
        labels+: labels,
      },
      data: {
        "admin_first_name": "Admin",
        "admin_last_name": "User",
        "blog_name": "Kubernetes blog!",
    }},

    secret: kube.Secret("wordpress") {
      metadata+: {
        labels+: labels,
      },
      data_+: {
        "user": "user",
        "password": "bitnami",
        "mail": "user@example.com",
    }},

    deployment: kube.Deployment("wordpress") {
      metadata+: {
        labels+: labels,
      },
      spec+: {
        template+: {
          spec+: {
            containers_+: {
              default: kube.Container("wordpress") {
                image: "bitnami/wordpress",
                ports_+: { http: { containerPort: 80 } },
                env_+: {
                  MARIADB_HOST: be.backend.master.service.metadata.name,
                  WORDPRESS_DATABASE_USER: kube.SecretKeyRef(be.backend.secret, "database_user"),
                  WORDPRESS_DATABASE_NAME: kube.SecretKeyRef(be.backend.secret, "database_name"),
                  WORDPRESS_DATABASE_PASSWORD: kube.SecretKeyRef(be.backend.secret, "database_password"),
                  WORDPRESS_USERNAME: kube.SecretKeyRef($.frontend.secret, "user"),
                  WORDPRESS_EMAIL: kube.SecretKeyRef($.frontend.secret, "mail"),
                  WORDPRESS_PASSWORD: kube.SecretKeyRef($.frontend.secret, "password"),
                  WORDPRESS_BLOG_NAME: kube.ConfigMapRef($.frontend.configmap, "blog_name"),
                  WORDPRESS_FIRST_NAME: kube.ConfigMapRef($.frontend.configmap, "admin_first_name"),
                  WORDPRESS_LAST_NAME: kube.ConfigMapRef($.frontend.configmap, "admin_last_name"),
                },
                livenessProbe: {
                  initialDelaySeconds: 120,
                  httpGet:  {
                    path: "/wp-login.php",
                    port: 80
                }},
                readinessProbe: self.livenessProbe {
                  initialDelaySeconds: 60,
                },
                volumeMounts_+: {
                  "wordpress-data": {
                    "mountPath": "/bitnami",
          }}}},
          volumes_+: {
            "wordpress-data": {
              "persistentVolumeClaim": {
                "claimName": "wordpress",
    }}}}}}},

    service: kube.Service("wordpress") {
      metadata+: {
        labels+: labels,
      },
      target_pod: $.frontend.deployment.spec.template,
}}}
