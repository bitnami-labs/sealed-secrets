## Description of the problem

Due the problems described here: <https://github.com/vmware-tanzu/kubeapps/issues/651> and licensing issues, MongoDB is no longer the best solution for our use case. Apart from that, even though is not the goal of this document, we are evaluating the possible effort needed to support other type of assets in Kubeapps (apart than charts) like operators.

The two items above, require a re-design of two micro-services: `chartsvc` and `chart-repo`. While the changes should be kept minimal, at least for the moment, we should plan in advance to support new asset types.

## Requisites

- High Availability. Avoid single point of failure.
- Open Source license. BSD, Apache2 or MIT are fine.
- Good performance. Even though the amount of data is not big, performance should be taken into account.
- Extensibility. The solution should be extendable to other kind of assets like operators.
- [_Good to have_] Backwards compatibility.

## Current usage

This section describes the current data we store in MongoDB and how we are using it in the different services.

### Collections

- `charts`: All the chart info. Including all the versions. ID: repo/chart. Include chartVersions. Each chartVersion includes date, version, appversion, digest, url to tarball and icon.
- `files`: Files related to a chart version. ID: repo/chart-version. Includes readme, schema, values, repo(object),
- `repos`: Latest sync for each repo. ID: repo. Includes checksum and date.

### Methods

chart-repo:

- repoAlreadyProcessed. Returns true if a checksum matches the one stored in `repos`.
- updateLastCheck. Update the date and checksum in `repos`.
- deleteRepo. Deletes a repository from `charts` and `files`.
- importCharts. Imports all the charts of a registry.
- fetchAndImportFiles. Store in `files` the files related to a chart version.

chartsvc:

- getPaginatedChartList. Gets all the charts (optionally from a single repo). It supports pagination and allows to filter out duplicated charts (e.g. stable/wordpress and bitnami/wordpress).
- getChart. Returns a single chart, including a reference to the latest version.
- listChartVersions. Returns a chart including all its chart versions.
- getChartVersion. Returns a single chart version.
- getChartIcon. Returns the icon from a chart (repo/chart). Supports different `IconContentTypes`. Note that this uses the `charts` collection.
- getChartVersionReadme. Returns the readme of a chart version from `files`.
- getChartVersionValues. Returns the values of a chart version from `files`.
- getChartVersionSchema. Returns the schema of a chart version from `files`.
- listChartsWithFilters. Returns the charts that matches a certain chart name, version and app version. Used to retrieve references to the repositories containing a chart.
- searchCharts. Given a term, return the list of charts that contains that term (regex) in the name, description, repository name, keywords, sources or maintainers.

## Alternatives

### 1. Kubernetes Native Resources

One option could be to simply use Kubernetes resources like ConfigMaps, Secrets or Custom Resources to store the data related to a chart in the cluster.

Pros:

- No need from external software.
- Protected by RBAC (as the rest of resources).
- Filtering/labelling from K8s can be reused.
- No need to handle upgrades/persistence.

Cons:

- etcd limit. No entry can be larger than 1.5MB. We cannot ensure that a chartVersion (or a logo) will eventually grow greater than that.
- Completely different approach. Backward compatibility cannot be ensured.
- We would still need a database for hub.kubeapps.com

### 2. Redis

Redis is a key/value object store with a high performance that can be used for our use case. Note that we would be storing JSON blobs as the value for different keys. BSD license.

Pros:

- High performance, can be run in HA mode (with several replicas).
- A chart with production values is already available.
- (also a Con) Data would be in JSON format. While this is easier to manage from a development point of view, its performance may not be that good since we need to serialize/deserialize these values.
- Minimal changes in the architecture of the data.
- It can be backwards compatible.

Cons:

- As described above, advanced queries cannot be done.
- We still need to handle persistence and upgrades.
- As in the previous option, we would still need a database for hub.kubeapps.com
- While there is [a plugin for JSON handling](https://redislabs.com/blog/redis-as-a-json-store/), it requires a different version of the official image so we won't be able to use it.

### 3. PostgreSQL

PostgreSQL is a relational database but it has support for [JSON data](https://www.postgresql.org/docs/current/functions-json.html) so we should be able to operate as we do today. PostgreSQL License.

Pros:

- High performance, can be run in HA mode (with several replicas).
- A chart with production values is already available.
- Official support for JSON objects, easier to migrate.
- Minimal changes in the architecture of the data.
- It can be backwards compatible.
- If needed, can be used in hub.kubeapps.com as well

Cons:

- We still need to handle persistence and upgrades.

### 4. FoundationDB Document Layer

[FoundationDB Document Layer](https://github.com/FoundationDB/fdb-document-layer) is an open source (Apache2 licensed) stateless microserver that exposes a document-oriented database API. It claims to be compatible with MongoDB API but using FoundationDB Key-Value store as the backend.

This would mean that we could potentially maintain the same API but there is not an offial helm chart and the official image is not maintained (last updated 6 month ago) so we can discard this option.

## Proposal

From the options listed above, the option that seems to fit our use case best with minimal impact is the point 3. Using PostgreSQL. Taking that into account, this is the list of task we would need to perform:

- Rename the `chartsvc` to something more generic: `assetsvc`. Move the code to this repository.
- Rename the `chart-repo` to something more generig: `asset-syncer`. Move the code to this repository.
- Adapt the CI to build and use those services.
- Adapt the chart to use those images instead.
- Isolate the actions that require a database connection and make them an interface.
- Implement that interface using PostgreSQL.
- Add the database choice in the chart.
- Update documentation.

To avoid breaking changes, we should default to use MongoDB. Once we release a new major version, we can swith the default to PostgreSQL. To avoid maintaining the two approaches we may decide to remove support for MongoDB at that point.
