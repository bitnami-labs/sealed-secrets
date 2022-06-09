# Kubeapps APIs service

The Kubeapps APIs service provides a pluggable, gRPC-based API service enabling the Kubeapps UI (or other clients) to interact with different Kubernetes packaging formats in a consistent, extensible way.

The Kubeapps APIs service is bundled with three packaging plugins providing support for the Helm, Carvel and Flux packaging formats, enabling users to browse and install packages of different formats.

![Kubeapps with packaging plugins](../../img/kubeapps-apis/packages-plugins.png)

In addition to these three packaging plugins, the Kubeapps APIs service is also bundled with a Kubernetes resources plugin that removes the long-standing requirement for the Kubeapps UI to talk directly with the Kubernetes API server. With this change, a user with the required RBAC can request, for example, Kubernetes resources for a specific installed package only:

![Kubeapps with resources plugins](../../img/kubeapps-apis/resources-plugin.png)

## Architectural overview

### A gRPC-based API server

We chose to use [gRPC/protobuf](https://grpc.io/) to manage our API definitions and implementations together with the [buf.build](https://buf.build/) tool for lint and other niceties. In that regard, it's a pretty standard stack using:

- [grpc-gateway](https://grpc-ecosystem.github.io/grpc-gateway/) to enable a RESTful JSON version of our API (we don't use this in our client, but not everyone uses gRPC either, so we want to ensure the API is accessible to others who would like to use it)
- Improbable's [grpc-web](https://github.com/improbable-eng/grpc-web) to enable TypeScript gRPC client generation as well as translating gRPC-web requests into plain gRPC calls in the backend (rather than requiring something heavier like [Envoy](https://grpc.io/docs/platforms/web/basics/#configure-the-envoy-proxy) to do the translation),
- We multiplex on a single port to serve gRPC, gRPC-web as well as JSON HTTP requests.

### A pluggable API server - loading plugins dynamically

A plugin for the Kubeapps APIs service is just a standard [Go plugin](https://pkg.go.dev/plugin) that exports two specific functions with the signatures:

```golang
func RegisterWithGRPCServer(GRPCPluginRegistrationOptions) (interface{}, error)

func RegisterHTTPHandlerFromEndpoint(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error
```

This allows the main `kubeapps-apis` service to load dynamically any plugins found in the specified plugin directories when the service starts. The startup process creates a gRPC server and then calls each plugin's `RegisterWithGRPCServer` function to ensure their functionality is served as part of the gRPC API and the `RegisterHTTPHandlerFromEndpoint` function to ensure that the same functionality is available via the gRPC-Gateway.

So for example, as you might expect, we have a `helm/v1alpha1` plugin that provides a helm catalog and the ability to install helm packages, as well as a `resources/v1alpha1` plugin which can be enabled to provide some access to Kubernetes resources, such as the resources related to an installed package (assuming the requestor has the correct RBAC) - more on that later.

With this structure, the kubeapps-apis executable loads the compiled plugin `.so` files from the plugin directories specified on the command-line and registers them when starting. You can find more details about the plugin registration functionality in the [core plugin implementation](https://github.com/vmware-tanzu/kubeapps/blob/main/cmd/kubeapps-apis/core/plugins/v1alpha1/plugins.go).

### An extensible API server - enabling different implementations of the core packages plugin

Where things become interesting is with the requirement to **support different Kubernetes packaging formats** via this pluggable system and **present them consistently to a UI** such as the Kubeapps dashboard.

To achieve this, we defined a core packages API (`core.packages.v1alpha1`) with an interface which any plugin can choose to implement. This interface consists of methods common to querying for and installing Kubernetes packages, such as `GetAvailablePackages` or `CreateInstalledPackage`. You can view the full protobuf definition of this interface
in the [packages.proto](https://github.com/vmware-tanzu/kubeapps/blob/main/cmd/kubeapps-apis/proto/kubeappsapis/core/packages/v1alpha1/packages.proto) file, but as an example, the `GetAvailablePackageDetail` RPC is defined as:

```protobuf
rpc GetAvailablePackageDetail(GetAvailablePackageDetailRequest) returns (GetAvailablePackageDetailResponse) {
  option (google.api.http) = {
    get: "/core/packages/v1alpha1/availablepackages/..."
  };
}
```

where the request looks like:

```protobuf
// GetAvailablePackageDetailRequest
//
// Request for GetAvailablePackageDetail
message GetAvailablePackageDetailRequest {
  // The information required to uniquely
  // identify an available package
  AvailablePackageReference available_package_ref = 1;

  // Optional specific version (or version reference) to request.
  // By default the latest version (or latest version matching the reference)
  // will be returned.
  string pkg_version = 2;
}
```

Similar to the normal Go idiom for [satisfying an interface](https://go.dev/doc/effective_go#interfaces), a Kubeapps APIs plugin satisfies the core packages interface if it implements all the methods of the core packages interface. So when the `kubeapps-apis` service's plugin server has registered all plugins, it subsequently iterates the set of plugins to see which of the registered plugins satisfy the core packages interface, returning a slice of packaging plugins satisfying the interface:

```golang
// GetPluginsSatisfyingInterface returns the registered plugins which satisfy a
// particular interface. Currently this is used to return the plugins that satisfy
// the core.packaging interface for the core packaging server.
func (s *pluginsServer) GetPluginsSatisfyingInterface(targetInterface reflect.Type) []PluginWithServer {
        satisfiedPlugins := []PluginWithServer{}
        for _, pluginSrv := range s.pluginsWithServers {
                // The following check if the service implements an interface is what
                // grpc-go itself does, see:
                // https://github.com/grpc/grpc-go/blob/v1.38.0/server.go#L621
                serverType := reflect.TypeOf(pluginSrv.Server)

                if serverType.Implements(targetInterface) {
                        satisfiedPlugins = append(satisfiedPlugins, pluginSrv)
                }
        }
        return satisfiedPlugins
}
```

Of course, all plugins register their own gRPC servers and so the RPC calls they define can be queried independently, but having a core packages interface and keeping a record of which plugins happen to satisfy the core packages interface allows us to ensure that **all plugins that support a different Kubernetes package format have a standard base API** for interacting with those packages, and importantly, the Kubeapps APIs services' core packages implementation can act as a gateway for all interactions, aggregating results for queries and generally proxying to the corresponding plugin.

### An Aggregated API server - combining results from different packaging plugins

Part of the goal of enabling pluggable support for different packaging systems is to ensure that a UI like the Kubeapps dashboard can use a single client to present a catalog of apps for install, regardless of whether they come from a standard Helm repository, or a flux-based Helm repository, or Carvel package resources on the cluster.

For this reason, the implementation of the core packages API delegates to the related packaging plugins and aggregates their results. For example, the core packages implementation of `GetAvailablePackageDetail` ([see `packages.go`](https://github.com/vmware-tanzu/kubeapps/blob/main/cmd/kubeapps-apis/core/packages/v1alpha1/packages.go)) can simply delegate to the relevant plugin:

```golang
// GetAvailablePackageDetail returns the package details based on the request.
func (s packagesServer) GetAvailablePackageDetail(ctx context.Context, request *packages.GetAvailablePackageDetailRequest) (*packages.GetAvailablePackageDetailResponse, error)
 {
        ...
        // Retrieve the plugin with server matching the requested plugin name
        pluginWithServer := s.getPluginWithServer(request.AvailablePackageRef.Plugin)
        ...

        // Get the response from the requested plugin
        response, err := pluginWithServer.server.GetAvailablePackageDetail(ctx, request)
        if err != nil {
          ...
        }


        // Build the response
        return &packages.GetAvailablePackageDetailResponse{
                AvailablePackageDetail: response.AvailablePackageDetail,
        }, nil
}
```

Similar implementations of querying functions like `GetAvailablePackageSummaries` in the same file collect the relevant available package summaries from each packaging plugin
and return the aggregated results. So our Kubeapps UI (or any UI using the client) can benefit from using the single _core_ packages client to query and interact with packages
from _different_ packaging systems, such as Carvel and Flux.

It is worth noting that a plugin that satisfies the core packages interface isn't restricted to _only_ those methods. Similar to go interfaces, the plugin is free to implement
other functionality in addition to the interface requirements. The Helm plugin uses this to include additional functionality for rolling back an installed package - something
which is not necessary for Carvel or Flux. This extra functionality is available on the Helm-specific gRPC client.

### Authentication/Authorization

Authentication-wise, we continue to rely on the OIDC standard so that every request that arrives at the Kubeapps APIs server must include a token to identify the user. This token is then relayed with requests to the Kubernetes API service on the users' behalf, ensuring that all use of the Kubernetes API server is with the users' configured RBAC. Each plugin receives a `core.KubernetesConfigGetter` function when being registered, which handles creating the required Kubernetes config for a given request context, so the plugin doesn't need to care about the details.

Note that although we don't support its use in anything other than a demo environment, a service account token can be used instead of a valid OIDC `id_token` to authenticate requests.

### Caveats

Although the current Kubeapps UI does indeed benefit from this core client and interacts with the packages from different packaging systems in a uniform way, we still have some exceptions to this. For example, Flux and Carvel require selecting a service account to be associated with the installed package. Rather than the plugin providing additional schema or field data for creating a package ([something we plan to add in the future](https://github.com/vmware-tanzu/kubeapps/issues/4365)), we've currently included the service account field based on the plugin name.

It's also worth noting that we tried and were unable to include any streaming gRPC calls on the core packages interface. While two separate packages can define the same interface (with the same methods, return types etc.), `grpc-go` generates package-specific types for streamed responses, which makes it impossible for one packages' implementation of a streaming RPC to match another one, such as the core interface. It is not impossible to work around this, but so far we've used streaming responses on other non-packages plugins, such as the resources plugin for reporting on the Kubernetes resources related to an installed package.

### Accessing K8s resources without exposing the Kubernetes API server

Since the beginning of the Kubeapps project, the Kubeapps dashboard required access to the Kubernetes API to be able to query and display the Kubernetes resources related to an installed package, as well as other functionality such as creating secrets or simply determining whether the user is authenticated (all of which required credentials with sufficient permissions). As a result, the Kubeapps frontend service has included a reverse proxy to the Kubernetes API since the very beginning. A major goal for the new `kubeapps-apis` service was to remove this reverse proxying of the Kubernetes API.

This was achieved by the creation of the `resources/v1alpha1` plugin, which provides a number of specific functions related to Kubernetes resources that are required by UIs such as the Kubeapps dashboard. For example, rather than being able to query (or update) resources via the Kubernetes API, the `resources/v1alpha1` plugin provides a [`GetResources` method that streams the resources](https://github.com/vmware-tanzu/kubeapps/blob/main/cmd/kubeapps-apis/proto/kubeappsapis/plugins/resources/v1alpha1/resources.proto) (or a subset thereof) for a specific installed package only:

```protobuf
// GetResourcesRequest
//
// Request for GetResources that specifies the resource references to get or watch.
message GetResourcesRequest {
    // InstalledPackageRef
    //
    // The installed package reference for which the resources are being fetched.
    kubeappsapis.core.packages.v1alpha1.InstalledPackageReference installed_package_ref = 1;

    // ResourceRefs
    //
    // The references to the resources that are to be fetched or watched.
    // If empty, all resources for the installed package are returned when only
    // getting the resources. It must be populated to watch resources to avoid
    // watching all resources unnecessarily.
    repeated kubeappsapis.core.packages.v1alpha1.ResourceRef resource_refs = 2;

    // Watch
    //
    // When true, this will cause the stream to remain open with updated
    // resources being sent as events are received from the Kubernetes API
    // server.
    bool watch = 3;
}
```

This enables a client such as the Kubeapps UI to request to watch a set of resources referenced by an installed package with a single request, with updates being returned for any resources in that set which change, which is much more efficient for the browser client than a watch request per resources sent previously sent to the Kubernetes API. Of course the implementation of the resources plugin still needs to issue a separate watch request per resource to the Kubernetes API, but it's much less of a problem to do this in our plugin than it is to do so from a web browser, which has set limits. Furthermore, it is much simpler to reason about with go channels since the messages from separate go channels of resource updates can be [merged into a single watcher](https://github.com/vmware-tanzu/kubeapps/blob/main/cmd/kubeapps-apis/plugins/resources/v1alpha1/server.go) with which to send data:

```golang
// Otherwise, if requested to watch the resources, merge the watchers
// into a single resourceWatcher and stream the data as it arrives.
resourceWatcher := mergeWatchers(watchers)
for e := range resourceWatcher.ResultChan() {
    sendResourceData(e.ResourceRef, e.Object, stream)
}
```

See the [`mergeWatchers` function](https://github.com/vmware-tanzu/kubeapps/blob/main/cmd/kubeapps-apis/plugins/resources/v1alpha1/server.go#L298-L335) for details of how the channel results are merged, which is itself inspired by the [fan-in example from the go blog](https://go.dev/blog/pipelines).

The resources plugin doesn't care which packaging system is used behind the scenes, all it needs to know is which packaging plugin is used so that it can verify the Kubernetes references for the installed package. In this way, the Kubeapps dashboard UI can present the Kubernetes resources for an installed package without caring which packaging system is involved.

## Trying it out

### The command-line interface

Similar to most go commands, we've used [Cobra](https://github.com/spf13/cobra) for the CLI interface. Currently, there is only a root command to run the server, but we may later add a `version` subcommand or a `new-plugin` subcommand, but even without these, it provides a lot of useful defaults for config, env var support, etc.

Although it is possible to run the service in isolation, it requires access to a cluster so it's much simpler to test the service via port-forwarding.

### Port-forwarding to the Kubeapps-API service

If you have custom changes you want to test, you can build the image from the kubeapps root directory with:

```bash
IMAGE_TAG=dev1 make kubeapps/kubeapps-apis
```

and make that image available on your cluster somehow. If using kind, you can simply do:

```bash
kind load docker-image kubeapps/kubeapps-apis:dev1 --name kubeapps
```

You can edit the values file to change the `kubeappsapis.image.tag` field to match the tag above, or edit the deployment once deployed to match, such as:

```bash
kubectl set image deployment/kubeapps-internal-kubeappsapis -n kubeapps kubeappsapis=kubeapps/kubeapps-apis:dev1 --record
```

With the kubeapps-apis service running, you can then test the packages endpoints in cluster by port-forwarding the service in one terminal:

```bash
kubectl -n kubeapps port-forward svc/kubeapps-internal-kubeappsapis 8080:8080
```

### Testing with cURL

You can then verify the configured plugins endpoint via http:

```bash
curl -s http://localhost:8080/apis/core/plugins/v1alpha1/configured-plugins | jq .
{
  "plugins": [
    {
      "name": "fluxv2.packages",
      "version": "v1alpha1"
    },
    {
      "name": "kapp_controller.packages",
      "version": "v1alpha1"
    },
    {
      "name": "resources",
      "version": "v1alpha1"
    }
  ]
}
```

or via gRPC (using the [grpcurl tool](https://github.com/fullstorydev/grpcurl)), note that the same host:port is used as we multiplex on the one port:

```bash
grpcurl -plaintext localhost:8080 kubeappsapis.core.plugins.v1alpha1.PluginsService.GetConfiguredPlugins
{
  "plugins": [
    {
      "name": "fluxv2.packages",
      "version": "v1alpha1"
    },
    {
      "name": "kapp_controller.packages",
      "version": "v1alpha1"
    },
    {
      "name": "resources",
      "version": "v1alpha1"
    }
  ]
}
```

### Testing an authenticated endpoint

You will need an authentication token to be able to query the API service's other endpoints, such as the packaging endpoints.

You can either create a service account with the necessary RBAC and use the related bearer token, or [steal the auth token from your browser](../../howto/OIDC/OAuth2OIDC-debugging.md#viewing-the-jwt-id-token). Either way, you will end up with a token that you can use with your queries. For example, to get the available packages:

```bash
$ export TOKEN="Bearer eyJhbGciO..."
$ curl -s http://localhost:8080/plugins/fluxv2/packages/v1alpha1/availablepackages -H "Authorization: $TOKEN" | jq . | head -n 26
{
  "availablePackageSummaries": [
    {
      "availablePackageRef": {
        "context": {
          "cluster": "default",
          "namespace": "default"
        },
        "identifier": "bitnami/airflow",
        "plugin": {
          "name": "fluxv2.packages",
          "version": "v1alpha1"
        }
      },
      "name": "airflow",
      "latestVersion": {
        "pkgVersion": "12.0.9",
        "appVersion": "2.2.3"
      },
      "iconUrl": "https://bitnami.com/assets/stacks/airflow/img/airflow-stack-220x234.png",
      "displayName": "airflow",
      "shortDescription": "Apache Airflow is a tool to express and execute workflows as directed acyclic graphs (DAGs). It includes utilities to schedule tasks, monitor task progress and handle task dependencies.",
      "categories": [
        "WorkFlow"
      ]
    },
```

Here is an example that shows how to use grpcurl to get the details on package "bitnami/apache" from the flux plugin

```bash
$ grpcurl -plaintext -d '{"available_package_ref": {"context": {"cluster": "default", "namespace": "default"}, "plugin": {"name": "fluxv2.packages", "version": "v1alpha1"}, "identifier": "bitnami/apache"}}' -H "Authorization: $TOKEN" localhost:8080 kubeappsapis.core.packages.v1alpha1.PackagesService.GetAvailablePackageDetail | jq . | head -n 23
{
  "availablePackageDetail": {
    "availablePackageRef": {
      "context": {
        "cluster": "default",
        "namespace": "default"
      },
      "identifier": "bitnami/apache",
      "plugin": {
        "name": "fluxv2.packages",
        "version": "v1alpha1"
      }
    },
    "name": "apache",
    "version": {
      "pkgVersion": "9.0.6",
      "appVersion": "2.4.52"
    },
    "repoUrl": "https://charts.bitnami.com/bitnami",
    "homeUrl": "https://github.com/bitnami/charts/tree/master/bitnami/apache",
    "iconUrl": "https://bitnami.com/assets/stacks/apache/img/apache-stack-220x234.png",
    "displayName": "apache",
    "shortDescription": "Apache HTTP Server is an open-source HTTP server. The goal of this project is to provide a secure, efficient and extensible server that provides HTTP services in sync with the current HTTP standards.",
```

Or you can query the core API to get an aggregation of all installed packages across the configured plugins:

```bash
grpcurl -plaintext -d '{"context": {"cluster": "default", "namespace": "kubeapps-user-namespace"}}' -H "Authorization: $TOKEN" localhost:8080 kubeappsapis.core.packages.v1alpha1.PackagesService.GetInstalledPackageSummaries | jq . | head -n 23
{
  "installedPackageSummaries": [
    {
      "installedPackageRef": {
        "context": {
          "cluster": "default",
          "namespace": "kubeapps-user-namespace"
        },
        "identifier": "apache-success",
        "plugin": {
          "name": "fluxv2.packages",
          "version": "v1alpha1"
        }
      },
      "name": "apache-success",
      "pkgVersionReference": {
        "version": "9.0.5"
      },
      "currentVersion": {
        "pkgVersion": "9.0.5",
        "appVersion": "2.4.52"
      },
      "iconUrl": "https://bitnami.com/assets/stacks/apache/img/apache-stack-220x234.png",
```

Of course, you will need to have the appropriate Flux HelmRepository or Carvel PackageRepository available. See [managing carvel packages](../../tutorials/managing-carvel-packages.md) or [managing flux packages](../../tutorials/managing-flux-packages.md) for information about setting up the environment.

## Hacking

A few extra tools will be needed to contribute to the development of this service.

### GOPATH env variable

Make sure your GOPATH environment variable is set.
You can use the value of command

```bash
go env GOPATH
```

### Install go cli deps

You should be able to install the exact versions of the various go CLI dependencies into your $GOPATH/bin with the following, after ensuring `$GOPATH/bin`is included in your`$PATH`:

```bash
make cli-dependencies
```

This will ensure that the cobra command is available should you need to add a sub-command.

### Install buf

Grab the latest binary from the [buf releases](https://github.com/bufbuild/buf/releases).

You can now try changing the url in the proto file (such as in `proto/kubeappsapis/core/v1/core.proto`) and then run:

```bash
buf generate
export KUBECONFIG=/home/user/.kube/config # replace it with your desired kube config file
make run
```

and then verify that the RegisteredPlugins RPC call is exposed via HTTP at the new URL path that you specified.

You can also use `buf lint` to ensure that the proto IDLs are valid (ie. extendable, no backwards incompatible changes etc.)
