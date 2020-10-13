// Copyright 2017 The kubecfg authors
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

// Simple to demonstrate kubecfg using kube-libsonnet
// This should not necessarily be considered a model jsonnet example
// to build upon.

// This is a simple port to jsonnet of the standard guestbook example
// https://github.com/kubernetes/kubernetes/tree/master/examples/guestbook
//
// ```
// kubecfg update guestbook.jsonnet
//
// # poke at 
// - $(minikube service frontend), etc
// - kubectl proxy # then visit http://localhost:8001/api/v1/namespaces/default/services/frontend/proxy/ 
// kubecfg delete guestbook.jsonnet
// ```

local kube = import "../../kube.libsonnet";

{
  frontend_deployment: kube.Deployment("frontend") {
    spec+: {
      local my_spec = self,
      replicas: 3,
      template+: {
        spec+: {
          containers_+: {
            gb_fe: kube.Container("gb-frontend") {
              image: "gcr.io/google-samples/gb-frontend:v4",
              resources: { requests: { cpu: "100m", memory: "100Mi" } },
              env_+: {
                GET_HOSTS_FROM: "dns",
                NUMBER_REPLICAS: my_spec.replicas,
              },
              ports_+: { http: { containerPort: 80 } },
  }}}}}},

  frontend_service: kube.Service("frontend") {
    target_pod: $.frontend_deployment.spec.template,
    // spec+: { type: "LoadBalancer" },
  },

  redis_master_deployment: kube.Deployment("redis-master") {
    spec+: {
      template+: {
        spec+: {
          containers_+: {
            redis_master: kube.Container("redis-master") {
              image: "gcr.io/google_containers/redis:e2e",
              resources: { requests: { cpu: "100m", memory: "100Mi" } },
              ports_+: {
                redis: { containerPort: 6379 },
  }}}}}}},

  redis_master_service: kube.Service("redis-master") {
    target_pod: $.redis_master_deployment.spec.template,
  },

  redis_slave_deployment: kube.Deployment("redis-slave") {
    spec+: {
      replicas: 2,
      template+: {
        spec+: {
          containers_+: {
            redis_slave: kube.Container("redist-slave") {
              image: "gcr.io/google_samples/gb-redisslave:v1",
              resources: {
                requests: { cpu: "100m", memory: "100Mi" },
              },
              env_: {
                GET_HOSTS_FROM: "dns",
              },
              ports_+: {
                redis: { containerPort: 6379 },
  }}}}}}},

  redis_slave_service: kube.Service("redis-slave") {
    target_pod: $.redis_slave_deployment.spec.template,
  },
}
