# End-to-end tests in the project

In every CI build, a set of end-to-end tests are run to verify, as much as possible, that the changes don't include regressions from a user point of view. Please refer to the [CI documentation](./ci.md) for further information.
The current end-to-end tests are just browser tests

These tests are run by the script [script/e2e-test.sh](https://github.com/vmware-tanzu/kubeapps/blob/main/script/e2e-test.sh). Particularly, this script:

1. Installs Kubeapps using the images built during the CI process (c.f., [CI config file](https://github.com/vmware-tanzu/kubeapps/blob/main/.circleci/config.yml)) by setting the proper args to the Helm command.
   1. If the `USE_MULTICLUSTER_OIDC_ENV` is enabled, a set of flags will be passed to configure the Kubeapps installation in a multicluster environment.
2. Waits for:
   1. the different deployments to be ready.
   2. the bitnami repo sync job to be completed.
3. Installs some dependencies:
   1. Chart Museum.
   2. Operator framework (not in GKE).
4. Runs the [web browser tests](#web-browser-tests).

If all of the above succeeded, the control is returned to the CI with the proper exit code.

## Web Browser tests

Apart from the basic functionality tests run by the chart tests, this project contains web browser tests that you can find in the [integration](https://github.com/vmware-tanzu/kubeapps/blob/main/integration) folder.

These tests are based on [Puppeteer](https://github.com/GoogleChrome/puppeteer). Puppeteer is a NodeJS library that provides a high-level API to control Chrome or Chromium (in headless mode by default).

On top of Puppeteer, we are using the `jest-puppeteer` module that allows us to run these tests using the same syntax as in the rest of the unit tests that we have in the project.

> NOTE: this information is now outdated. We are using [playwright](https://playwright.dev) instead. This documentation will be eventually updated accordingly.

The aforementioned [integration](https://github.com/vmware-tanzu/kubeapps/blob/main/integration) folder is self-contained, that is, it contains every required dependency to run the browser tests in a separate [package.json](https://github.com/vmware-tanzu/kubeapps/blob/main/integration/package.json). Furthermore, a [Dockerfile](https://github.com/vmware-tanzu/kubeapps/blob/main/integration/Dockerfile) is used to generate an image with [all the dependencies](https://github.com/puppeteer/puppeteer/blob/main/docs/troubleshooting.md#chrome-headless-doesnt-launch-on-unix) needed to run the browser tests.

These tests can be run either [locally](#running-browser-tests-locally) or in a [container environment](#running-browser-tests-in-a-pod).

You can set up a configured Kubeapps instance in your cluster with the [script/setup-kubeapps.sh](https://github.com/vmware-tanzu/kubeapps/blob/main/site/content/docs/latest/reference/scripts/setup-kubeapps.sh) script.

### Running browser tests locally

To run the tests locally you just need to install the required dependencies and set the required environment variables:

```bash
cd integration
yarn install
INTEGRATION_ENTRYPOINT=http://kubeapps.local USE_MULTICLUSTER_OIDC_ENV=false ADMIN_TOKEN=foo1 VIEW_TOKEN=foo2 EDIT_TOKEN=foo3 yarn start

```

If a test happens to fail, besides the test logs, a screenshot will be generated and saved in the `reports/screenshots` folder.

### Running browser tests in a pod

Since the CI environment doesn't have the required dependencies and to provide a reproducible environment, it's possible to run the browser tests in a Kubernetes pod.

To do so, you can spin up an instance running the image [kubeapps/integration-tests](https://hub.docker.com/r/kubeapps/integration-tests).
This image contains all the required dependencies and it waits forever so you can run commands within it.
We also provide a simple [Kubernetes Deployment manifest](https://github.com/vmware-tanzu/kubeapps/blob/main/integration/manifests/executor.yaml) for launching this container.

The goal of this setup is that you can copy the latest tests to the image, run the tests and extract the screenshots in case of failure:

```bash
cd integration

# Deploy the executor pod
kubectl apply -f manifests/executor.yaml
pod=$(kubectl get po -l run=integration -o jsonpath="{.items[0].metadata.name}")

# Copy latest tests
kubectl cp ./tests ${pod}:/app/

# If you also modify the test configuration, you will need to update the files
# for f in *.js; do   kubectl cp "./${f}" "${pod}:/app/"; done


# Run tests (you must fill these vars accordingly)
kubectl exec -it ${pod} -- /bin/sh -c "INTEGRATION_ENTRYPOINT=http://kubeapps.kubeapps USE_MULTICLUSTER_OIDC_ENV=${USE_MULTICLUSTER_OIDC_ENV} ADMIN_TOKEN=${admin_token} VIEW_TOKEN=${view_token} EDIT_TOKEN=${edit_token} yarn start"

# If the tests fail, get report screenshot
kubectl cp ${pod}:/app/reports ./reports
```

#### Building the "kubeapps/integration-tests" image

Our CI system relies on the [kubeapps/integration-tests](https://hub.docker.com/r/kubeapps/integration-tests) image to run browser tests (c.f., [CI config file](https://github.com/vmware-tanzu/kubeapps/blob/main/.circleci/config.yml) and [CI documentation](./ci.md)). Consequently, this image should be properly versioned to avoid CI issues.

The `kubeapps/integration-tests` image is built using this [Makefile](https://github.com/vmware-tanzu/kubeapps/blob/main/integration/Makefile), it uses the `IMAGE_TAG` variable to pass the version with which the image is built. It is important to increase the version each time the image is built and pushed:

```bash
# Get the latest tag from https://hub.docker.com/r/kubeapps/integration-tests/tags?page=1&ordering=last_updated
# and then increment the patch version of the latest tag to get the IMAGE_TAG that you'll use below.
cd integration
IMAGE_TAG=v1.0.1 make build
IMAGE_TAG=v1.0.1 make push
```

> It will build and push the image using this [Dockerfile](https://github.com/vmware-tanzu/kubeapps/blob/main/integration/Dockerfile) (we are using the base image as in the [Kubeapps Dashboard build image](https://github.com/vmware-tanzu/kubeapps/blob/main/dashboard/Dockerfile)).
> The dependencies of this image are defined in the [package.json](https://github.com/vmware-tanzu/kubeapps/blob/main/integration/package.json).

Then, update the [Kubernetes Deployment manifest](https://github.com/vmware-tanzu/kubeapps/blob/main/integration/manifests/executor.yaml) to point to the version you have built and pushed.

To sum up, whenever a change triggers a new `kubeapps/integration-tests` version (new NodeJS image, updating the integration dependencies, other changes, etc.), you will have to release a new version. This process involves:

- Checking if the [integration Dockerfile](https://github.com/vmware-tanzu/kubeapps/blob/main/integration/Dockerfile) is using the proper base version.
- Ensuring we are not using any deprecated dependency in the [package.json](https://github.com/vmware-tanzu/kubeapps/blob/main/integration/package.json).
- Updating the [Makefile](https://github.com/vmware-tanzu/kubeapps/blob/main/integration/Makefile) with the new version tag.
- running `make build && make push` to release a new image version.
- Modifying the [Kubernetes Deployment manifest](https://github.com/vmware-tanzu/kubeapps/blob/main/integration/manifests/executor.yaml) with the new version.
