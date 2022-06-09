# Kubeapps Releases Developer Guide

The purpose of this document is to guide you through the process of releasing a new version of Kubeapps.

## 0 - Ensure all 3rd-party dependencies are up to date

This step aims at decreasing the number of outdated dependencies so that we can get the latest patches with bug and security fixes.
It consists of four main stages: update the development images, update the CI, update the chart and, finally, update the dependencies.

### 0.1 - Development images

For building the [development container images](https://hub.docker.com/u/kubeapps), a number of base images are used in the build stage. Specifically:

- The [dashboard/Dockerfile](https://github.com/vmware-tanzu/kubeapps/blob/main/dashboard/Dockerfile) uses:
  - [bitnami/node](https://hub.docker.com/r/bitnami/node/tags) for building the static files for production.
  - [bitnami/nginx](https://hub.docker.com/r/bitnami/nginx/tags) for serving the HTML and JS files as a simple web server.
- Those services written in Golang use the same image for building the binary, but then a [scratch](https://hub.docker.com/_/scratch) image is used for actually running it. These Dockerfiles are:
  - [apprepository-controller/Dockerfile](https://github.com/vmware-tanzu/kubeapps/blob/main/cmd/apprepository-controller/Dockerfile).
  - [asset-syncer/Dockerfile](https://github.com/vmware-tanzu/kubeapps/blob/main/cmd/asset-syncer/Dockerfile).
  - [assetsvc/Dockerfile](https://github.com/vmware-tanzu/kubeapps/blob/main/cmd/assetsvc/Dockerfile).
  - [kubeops/Dockerfile](https://github.com/vmware-tanzu/kubeapps/blob/main/cmd/kubeops/Dockerfile).
- The [pinniped-proxy/Dockerfile](https://github.com/vmware-tanzu/kubeapps/blob/main/cmd/pinniped-proxy/Dockerfile) uses:
  - [\_/rust](https://hub.docker.com/_/rust) for building the binary.
  - [bitnami/minideb](https://hub.docker.com/r/bitnami/minideb) for running it.

> As part of this release process, these image tags _must_ be updated to the latest minor/patch version. In case of a major version, the change _should_ be tracked in a separate PR.
> **Note**: as the official container images are those being created by Bitnami, we _should_ ensure that we are using the same major version as they are using.

### 0.2 - CI configuration and images

In order to be in sync with the container images while running the different CI jobs, it is necessary to also update the CI image versions.
Find further information in the [CI configuration](../testing/ci.md) and the [e2e tests documentation](../testing/end-to-end-tests.md).

#### 0.2.1 - CI configuration

In the [CircleCI configuration](https://github.com/vmware-tanzu/kubeapps/blob/main/.circleci/config.yml) we have an initial declaration of the variables used along with the file.
The versions used there _must_ match the ones used for building the container images. Consequently, these variables _must_ be changed accordingly:

- `GOLANG_VERSION` _must_ match the versions used by our services written in Golang, for instance, [kubeops](https://github.com/vmware-tanzu/kubeapps/blob/main/cmd/kubeops/Dockerfile).
- `NODE_VERSION` _must_ match the **major** version used by the [dashboard](https://github.com/vmware-tanzu/kubeapps/blob/main/dashboard/Dockerfile).
- `RUST_VERSION` _must_ match the version used by the [pinniped-proxy](https://github.com/vmware-tanzu/kubeapps/blob/main/dashboard/Dockerfile).
- `DOCKER_VERSION` can be updated to the [latest version provided by CircleCI](https://circleci.com/docs/2.0/building-docker-images/#docker-version).
- `HELM_VERSION_MIN` _must_ match the one listed in the [Bitnami Application Catalog prerequisites](https://github.com/bitnami/charts#prerequisites).
- `HELM_VERSION_STABLE` should be updated with the [latest stable version from the Helm releases](https://github.com/helm/helm/releases).
- `OLM_VERSION` should be updated with the [latest stable version from the OLM releases](https://github.com/operator-framework/operator-lifecycle-manager/releases).
- `KAPP_CONTROLLER_VERSION` should be updated with the [latest stable version from the Kapp Controller releases](https://github.com/vmware-tanzu/carvel-kapp-controller/releases).
- `MKCERT_VERSION` should be updated with the [latest stable version from the mkcert releases](https://github.com/FiloSottile/mkcert/releases).
- `KUBECTL_VERSION` _should_ match the Kubernetes minor version (or minor version +1) used in `GKE_REGULAR_VERSION_XX` and listed in the [Kubernetes releases page](https://kubernetes.io/releases/).
- `GITHUB_VERSION` should be updated with the [latest stable version from the GitHub CLI releases](https://github.com/cli/cli/releases).
- `SEMVER_VERSION` should be updated with the [latest stable version from the semver releases](https://github.com/fsaintjacques/semver-tool/releases/tag/3.3.0).
- `KIND_VERSION` should be updated with the [latest stable version from the kind releases](https://github.com/kubernetes-sigs/kind/releases).
- `K8S_KIND_VERSION` _must_ match the Kubernetes minor version used in `GKE_REGULAR_VERSION_XX` and should be updated with one of the available image tags for a given [Kind release](https://github.com/kubernetes-sigs/kind/releases).
- `POSTGRESQL_VERSION` _must_ match the version used by the [Bitnami PostgreSQL chart](https://github.com/bitnami/charts/blob/master/bitnami/postgresql/values.yaml).
- `DEFAULT_MACHINE_IMG` _should_ be up to date according to the [list of available machines in CircleCI](https://circleci.com/docs/2.0/configuration-reference/#available-linux-machine-images).

Besides, the `GKE_STABLE_VERSION_XX` and the `GKE_REGULAR_VERSION_XX` might have to be updated if the _Stable_ and _Regular_ Kubernetes versions in GKE have changed. Check this information on [this GKE release notes website](https://cloud.google.com/kubernetes-engine/docs/release-notes).

> **NOTE**: at least one of those `GKE_STABLE_VERSION_XX` or `GKE_REGULAR_VERSION_XX` versions _must_ match the Kubernetes-related dependencies in [Go](https://github.com/vmware-tanzu/kubeapps/blob/maingo.mod) and [Rust](https://github.com/vmware-tanzu/kubeapps/blob/main/cmd/pinniped-proxy/Cargo.toml).
> As part of this release process, these variables _must_ be updated accordingly. Other variable changes _should_ be tracked in a separate PR.

#### 0.2.2 - CI integration image and dependencies

We use a separate integration image for running the e2e tests consisting of a simple Node image with a set of dependencies. Therefore, upgrading it includes:

- The [integration dependencies](https://github.com/vmware-tanzu/kubeapps/blob/main/integration/package.json) can be updated by running:

```bash
cd integration
yarn upgrade
```

- The [integration/Dockerfile](https://github.com/vmware-tanzu/kubeapps/blob/main/integration/Dockerfile) uses a [bitnami/node](https://hub.docker.com/r/bitnami/node/tags) image for running the e2e tests.

> As part of this release process, this Node image tag _may_ be updated to the latest minor/patch version. In case of a major version, the change _should_ be tracked in a separate PR. Analogously, its dependencies _may_ also be updated, but in case of a major change, it _should_ be tracked in a separate PR.
> **Note**: this image is not being built automatically. Consequently, a [manual build process](../testing/end-to-end-tests.md#building-the-kubeappsintegration-tests-image) _must_ be triggered if you happen to upgrade the integration image or its dependencies.

### 0.3 - Protobuf dependencies and autogenerated code

As per the introduction of the new Kubeapps APIs service, it is based upon automatic code generation for both the frontend code and backend code. Given that generation rules can evolve to improve or reduce possible bugs, it is important to perform a periodic update.

- To upgrade the `buf`-related dependencies, just run:

```bash
# You need to have the latest buf binary installed, if not, go to https://docs.buf.build/installation/
buf mod update  cmd/kubeapps-apis/
```

- Next, the autogenerated code ought to be regenerated. Note that some of the fronted code files might not comply with the `prettier` rules, therefore, triggering the linter may be required.

```bash
# You need to have the latest buf binary installed, if not, go to https://docs.buf.build/installation/
cd  cmd/kubeapps-apis
make generate
npx prettier --write  ../../dashboard/src/
```

> As part of this release process, the buf.lock dependencies _must_ be updated to the latest versions. In case of a major version, the change _should_ be tracked in a separate PR.

### 0.4 - Upgrading the code dependencies

Currently, we have three types of dependencies: the [dashboard dependencies](https://github.com/vmware-tanzu/kubeapps/blob/main/dashboard/package.json), the [golang dependencies](https://github.com/vmware-tanzu/kubeapps/blob/maingo.mod), and the [rust dependencies](https://github.com/vmware-tanzu/kubeapps/blob/main/cmd/pinniped-proxy/Cargo.toml). They _must_ be upgraded to the latest minor/patch version to get the latest bug and security fixes.

#### Dashboard dependencies

Upgrade the [dashboard dependencies](https://github.com/vmware-tanzu/kubeapps/blob/main/dashboard/package.json) by running:

```bash
cd dashboard
yarn upgrade
```

Note: If there are certain dependencies which cannot be updated currently, `yarn upgrade-interactive` allows selecting just certain items for upgrade.

#### Golang dependencies

Check the outdated [golang dependencies](https://github.com/vmware-tanzu/kubeapps/blob/maingo.mod) by running the following (from [How to upgrade and downgrade dependencies](https://github.com/golang/go/wiki/Modules#how-to-upgrade-and-downgrade-dependencies)):

```bash
go mod tidy
go list -u -f '{{if (and (not (or .Main .Indirect)) .Update)}}{{.Path}}: {{.Version}} -> {{.Update.Version}}{{end}}' -m all 2> /dev/null
```

Then, try updating to the latest version for all direct and indirect dependencies of the current module running this command:

```bash
go get -u ./...
```

> In case this above command fails (for example, due to an issue with transitive dependencies), you can manually upgrade those versions. A useful tool for doing so is [go-mod-upgrade](https://github.com/oligot/go-mod-upgrade).

#### Rust dependencies

Upgrade the [rust dependencies](https://github.com/vmware-tanzu/kubeapps/blob/main/cmd/pinniped-proxy/Cargo.toml) by running:

```bash
cd cmd/pinniped-proxy/
cargo update
```

#### Security and chart sync PRs

Finally, look at the [pull requests](https://github.com/vmware-tanzu/kubeapps/pulls) and ensure there is no PR open by Snyk or `kubeapps-bot` fixing a security issue or bringing upstream chart changes. If so, discuss it with another Kubeapps maintainer and come to a decision on it, trying not to release with a high/medium severity issue.

> As part of this release process, the dashboard deps _must_ be updated, the golang deps _should_ be updated, the rust deps _should_ be updated and the security check _must_ be performed.

#### Send a PR with the upgrades

Now create a Pull Request containing all these changes (only if no major versions have been bumped up) and wait until for another Kubeapps maintainer to review and accept so you can merge it.

## 1 - Select the commit to be tagged and perform some tests

Once the dependencies have been updated and the chart changes merged, the next step is to choose the proper commit so that we can base the release on it. It is, usually, the latest commit in the main branch. Then, some manual and automated tests should be performed to ensure the stability and reliability of the release.

## 1.1 - Trigger a `prerelease` CI flow

One of the CI flows we have defined is the `prerelease` one. It is being triggered once a commit is pushed to the `prerelease` branch in the Kubeapps repository. Although the precise instructions may differ depending on your git configuration, the main steps are:

```bash
# assuming you are in the main branch, with the latest changes pulled locally
# and you already have a `prerelease` local branch
git checkout prerelease
git merge main
git push origin prerelease # replace `origin` by your remote name
```

Then, check out the workflow that has just been created in CircleCI: [https://app.circleci.com/pipelines/github/vmware-tanzu/kubeapps?branch=prerelease](https://app.circleci.com/pipelines/github/vmware-tanzu/kubeapps?branch=prerelease).

## 1.2 - Perform a manual test

Even though we have a thorough test suite in our repository, we still _must_ perform a manual review of the application as it is in the selected commit. To do so, follow these instructions:

- Perform a checkout of the chosen commit.
- Install Kubeapps using the development chart: `helm install kubeapps ./chart/kubeapps/ -n kubeapps`
  - Note that if you are not using the latest commit in the main branch, you may have to locally build the container images so that the cluster uses the proper images.
- Ensure the core functionality is working:
  - Add a repository
  - Install an application from the catalog
  - Upgrade this application
  - Delete this application
  - Deploy an application in an additional cluster

## 2 - Create a git tag

Next, create a tag for the aforementioned commit and push it to the main branch. Please note that the tag name will be used as the release name.

For doing so, run the following commands:

```bash
export VERSION_NAME="v1.0.0-beta.1" # edit it accordingly

git tag ${VERSION_NAME} -m ${VERSION_NAME}
git push origin ${VERSION_NAME} # replace `origin` by your remote name
```

> You can retrieve the `VERSION_NAME` using the [semver tool](https://github.com/fsaintjacques/semver-tool) for properly increasing the version from the latest pushed tag:
>
> ```bash
> export VERSION_NAME="v$(semver bump <major|minor|patch> $(git fetch --tags && git describe --tags $(git rev-list --tags --max-count=1)))"
> ```

A new tag pushed to the repository will trigger, apart from the usual test and build steps, a _release_ [workflow](https://circleci.com/gh/kubeapps/workflows) as described in the [CI documentation](../testing/ci.md). An example of the triggered workflow is depicted below:

![CircleCI workflow after pushing a new tag](../../img/ci-workflow-release.png "CircleCI workflow after pushing a new tag")

> When a new tag is detected, Bitnami will automatically build a set of container images based on the tagged commit. They later will be published in [the Bitnami Dockerhub image registry](https://hub.docker.com/search?q=bitnami%2Fkubeapps&type=image).
> Please note that this workflow is run outside the control of the Kubeapps release process

## 3 - Complete the GitHub release notes

Once the release job is finished, you will have a pre-populated [draft GitHub release](https://github.com/vmware-tanzu/kubeapps/releases).

You still _must_ add a high-level description with the release highlights. Please take apart those commits just bumping dependencies up; it may prevent important commits from being clearly identified by our users.

Then, save the draft and **do not publish it yet** and get these notes reviewed by another Kubeapps maintainer.

## 4 - Manually review the PR created in the bitnami/charts repository

Since the chart that we host in the Kubeapps repository is only intended for development purposes, we need to synchronize it with the official one in the [bitnami/charts repository](https://github.com/bitnami/charts/tree/master/bitnami/kubeapps).

To this end, our CI system will automatically (in the `sync_chart_to_bitnami` workflow, as described in the [CI documentation](../testing/ci.md).) send a PR with the current development changes to [their repository](https://github.com/bitnami/charts/pulls) whenever a new release is triggered.
Once the PR has been created, have a look at it (eg. remove any development changes that should not be released) and wait for someone from the Bitnami team to review and accept it.

> Some issues can arise here, so please check the app versions are being properly updated at once and ensure you have the latest changes in the PR branch. Note that the [bitnami-bot](https://github.com/bitnami-bot) usually performs some automated commits to the main branch that might collide with the changes in our PR. In particular, it will release a new version of the chart with the updated images.

## 5 - Check Dockerfiles and notify the proper teams

Eventually, as the Kubeapps code continues to evolve, some changes are often introduced in our own [development container images](https://hub.docker.com/u/kubeapps). However, those changes won't get released in the official Bitnami repository unless we manually notify the proper team to also include those changes in their building system.

> As part of this release process, each Kubeapps component's Dockerfile _must_ compared against the one in the previous release. If they functionally differ each other, the Bitnami Content team _must_ be notified.

## 6 - Check released version is in Bitnami repository

Make sure the version is now publicly available in Bitnami repository.
The correct app and chart versions should appear when performing a search:

```bash
helm repo update && helm search repo kubeapps
```

## 7 - Publish the GitHub release

Once the new version of the [Kubeapps official chart](https://github.com/bitnami/charts/tree/master/bitnami/kubeapps) has been published and the release notes reviewed, you are ready to publish the release by clicking on the _publish_ button in the [GitHub releases page](https://github.com/vmware-tanzu/kubeapps/releases).

> Take into account that the chart version will be eventually published as part of the usual Bitnami release cycle. So expect this step to take a certain amount of time.

## 8 - Promote the release

Tell the community about the new release by using our Kubernetes slack [#kubeapps channel](https://kubernetes.slack.com/messages/kubeapps). If it includes major features, you might consider promoting it on social media.
