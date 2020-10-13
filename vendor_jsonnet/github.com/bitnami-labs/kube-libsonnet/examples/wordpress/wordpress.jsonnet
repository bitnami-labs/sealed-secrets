// Copyright (c) 2018 Bitnami
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

// ```
// kubecfg update wordpress.jsonnet
//
// kubecfg delete wordpress.jsonnet
// ```

local kube = import "../../kube.libsonnet";
local fe = import "frontend.jsonnet";
local be = import "backend.jsonnet";

local findObjs(top) = std.flattenArrays([
  if (std.objectHas(v, "apiVersion") && std.objectHas(v, "kind")) then [v] else findObjs(v)
  for v in kube.objectValues(top)
]);

kube.List() {
  items_+: {
    frontend: fe,
    backend: be,
  },
  items: findObjs(self.items_),
}
