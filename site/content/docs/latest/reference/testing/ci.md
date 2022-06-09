# Understanding the CircleCI configuration

Kubeapps leverages CircleCI for running the tests (both unit and integration tests), pushing the images and syncing the chart with the official [Bitnami chart](https://github.com/bitnami/charts/tree/master/bitnami/kubeapps). The following image depicts how a successful workflow looks like after pushing a commit to the main branch.

![CircleCI workflow after pushing to the main branch](../../img/ci-workflow-main.png "CircleCI workflow after pushing to the main branch")

The main configuration is located at this [CircleCI config file](https://github.com/vmware-tanzu/kubeapps/blob/main/.circleci/config.yml). At a glance, it contains:

- **Build conditions**: `build_always`, `build_on_main`, `build_on_tag` and `build_on_tag_or_prerelease`. They will be added to each job to determine whether or not it should be run. Whereas some should always be run, others only make sense when pushing to the main branch or when a new tag has been created.
- **Workflows**: we only use a single workflow named `kubeapps` with multiple jobs.
- **Jobs**: the actual commands that are run depending on the build conditions.
  - `test_go` (always): it runs every unit test for those projects written in Golang (that is, it runs `make test`) as well as it runs some DB-dependent tests.
  - `test_dashboard` (always): it runs the dashboard linter and unit tests (`yarn lint` and `yarn test`)
  - `test_pinniped_proxy` (always): it runs the Rust unit tests of the pinniped-proxy project (`cargo test`).
  - `build_go_images` (always): it builds the CI golang images for `kubeops`, `apprepository-controller`, `asset-syncer` and `assetsvc`.
  - `build_dashboard` (always): it builds the CI node image for `dashboard`.
  - `build_pinniped_proxy` (always): it builds the CI rust image for `pinniped-proxy`.
  - `local_e2e_tests` (always): it runs locally (i.e., inside the CircleCI environment) the e2e tests. Please refer to the [e2e tests documentation](./end-to-end-tests.md) for further information. In this job, before running the script [`script/e2e-test.sh](https://github.com/vmware-tanzu/kubeapps/blob/main/script/e2e-test.sh), the proper environment is created. Namely:
    - Install the required binaries (kind, kubectl, mkcert, helm).
    - Spin up two Kind clusters.
    - Load the CI images into the cluster.
    - Run the integration tests.
  - `sync_chart_from_bitnami` (on main): each time a new commit is pushed to the main branch, it brings the current changes in the upstream [bitnami/charts repository](https://github.com/bitnami/charts/tree/master/bitnami/kubeapps) and merges the changes. This step involves:
    - Checking if the Bitnami chart version is greater than the Kubeapps development chart version. If not, stop.
    - Deleting the local `chart/kubeapps` folder (note that the changes are already committed in git).
    - Cloning the fork [kubeapps-bot/charts repository](https://github.com/kubeapps-bot/charts/tree/master/bitnami/kubeapps), pulling the latest upstream changes and pushing them back to the fork.
    - Retrieving the latest version of the chart provided by Bitnami.
    - Renaming the production images (`bitnami/kubeapps-xxx`) by the development ones (`kubeapps/xxx`) with the `latest` tag.
    - Using `DEVEL` as the `appVersion`.
    - Sending a draft PR in the Kubeapps repository with these changes (from a pushed branch in the Kubeapps repository).
  - `push_images` (on main): the CI images (which have already been built) get re-tagged and pushed to the `kubeapps` account.
  - `GKE_STABLE_VERSION_MAIN` and `GKE_STABLE_VERSION_LATEST_RELEASE` (on tag or prerelease): there is a job for each [Kubernetes version (stable and regular) supported by Google Kubernetes Engine](https://cloud.google.com/kubernetes-engine/docs/release-notes) (GKE). It will run the e2e tests in a GKE cluster (version X.XX) using either the code in `prerelease` or in the latest released version. If a change affecting the UI is pushed to the main branch, the e2e test might fail here. Use a try/catch block to temporarily work around this.
  - `GKE_REGULAR_VERSION_MAIN` and `GKE_REGULAR_VERSION_LATEST_RELEASE` (on tag or prerelease): the same as above, but using the Kubernetes regular version in GKE.
  - `sync_chart_to_bitnami` (on tag): when releasing, it will synchronize our development chart with the [bitnami/charts repository](https://github.com/bitnami/charts/tree/master/bitnami/kubeapps) and merge the changes. This step involves:
    - Checking if the Kubeapps development chart version is greater than the Bitnami chart version. If not, stop.
    - Deleting the local `bitnami/kubeapps` folder (note that the changes are already committed in git).
    - Cloning the fork [kubeapps-bot/charts repository](https://github.com/kubeapps-bot/charts/tree/master/bitnami/kubeapps), pulling the latest upstream changes and pushing them back to the fork.
    - Retrieving the latest version of the chart provided by Kubeapps.
    - Renaming the development images (`kubeapps/xxx`) by the production ones (`bitnami/kubeapps-xxx`) with the `vX.X.X` tag.
    - Using `vX.X.X` as the `appVersion`.
    - Sending a draft PR to the Bitnami Charts repository with these changes (from the robot account's personal fork)
  - `release` (on tag): it creates a GitHub release based on the current tag by running the script [script/create_release.sh](https://github.com/vmware-tanzu/kubeapps/blob/main/script/create_release.sh).

Note that this process is independent of the release of the official Bitnami images and chart. These Bitnami images will be created according to their internal process (so the Golang, Node or Rust versions we define here are not used by them. Manual coordination is expected here if a major version bump happens to occur).

Also, note it is the Kubeapps team that is responsible for sending a PR to the [chart repository](https://github.com/bitnami/charts/tree/master/bitnami/kubeapps) each time a new chart version is to be released. Even this process is automatic (using the `sync_chart_to_bitnami` workflow), Kubeapps maintainers must manually review the draft PR and convert it into a normal one once it is ready for review.

# Credentials

Besides other usual credentials or secrets passed through environment variables via the CircleCI user interface, it is important to highlight how we grant commit and PR access to our robot account `kubeapps-bot <tanzu-kubeapps-team@vmware.com>`. The process is threefold:

- Create a [personal access token](https://docs.github.com/en/github/authenticating-to-github/creating-a-personal-access-token) with the robot account, granted, at least, with: `repo:status`, `public_repo` and `read:org`. This token must be stored as the environment variable `GITHUB_TOKEN` (is where Github CLI will look for)
  - That will allow the GitHub CLI to create PRs from the command line on behalf of our robot account.
  - Also, this token will be used for performing authenticated GitHub API calls.
- Add deployment keys to the repositories to which the CI will commit. Currently, they are `vmware-tanzu/kubeapps` and `kubeapps-bot/charts`.
  - This step allows the robot account to push branches remotely. However, the CI will never push to the main branch as it always tries to create a pull request.
- Add the robot account GPG key pair in the `GPG_KEY_PUBLIC` and `GPG_KEY_PRIVATE` environment variables.
  - The public key must be also uploaded in the robot account GPG settings in GitHub. It will be used for signing the commits and tags created by this account.

## Generating and configuring the deployment keys

This step is only run once, and it is very unlikely to change. However, it is important to know it in case of secret rotations or further events.

```bash
# COPY THIS CONTENT TO GITHUB (with write access):
## https://github.com/vmware-tanzu/kubeapps/settings/keys
ssh-keygen -t ed25519 -C "tanzu-kubeapps-team@vmware.com" -q -N "" -f circleci-kubeapps-deploymentkey
echo "Kubeapps deployment key (public)"
cat circleci-kubeapps-deploymentkey.pub

# COPY THIS CONTENT TO GITHUB (with write access):
## https://github.com/kubeapps-bot/charts/settings/keys
ssh-keygen -t ed25519 -C "tanzu-kubeapps-team@vmware.com" -q -N "" -f circleci-charts-deploymentkey
echo "Charts deployment key (public)"
cat circleci-charts-deploymentkey.pub

# COPY THIS CONTENT TO CIRCLECI (hostname: github.com):
## https://app.circleci.com/settings/project/github/vmware-tanzu/kubeapps/ssh
echo "Kubeapps deployment key (private)"
cat circleci-kubeapps-deploymentkey

echo "Charts deployment key (private)"
cat circleci-charts-deploymentkey

# COPY THE FINGERPRINTS TO ".circleci/config.yml"
## sync_chart_from_bitnami
echo "Charts deployment key (fingerprint) - edit 'sync_chart_from_bitnami'"
ssh-keygen -l -E md5 -f circleci-kubeapps-deploymentkey.pub

## sync_chart_to_bitnami
echo "Charts deployment key (fingerprint) - edit 'sync_chart_to_bitnami'"
ssh-keygen -l -E md5 -f circleci-charts-deploymentkey.pub
```

## Debugging the CI errors

As per the official [CircleCI documentation](https://circleci.com/docs/2.0/ssh-access-jobs/), one of the best ways to troubleshoot problems is to SSH into a job and inspect things like log files, running processes, and directory paths. For doing so, you have to:

- Ensure that you have added an SSH key to your GitHub account.

- Start a job with SSH enabled, that is, select the _Rerun job with SSH_ option from the _Rerun Workflow_ dropdown menu.

  - To see the connection details, expand the _Enable SSH_ section in the job output where you will see the SSH command needed to connect.

The build will remain available for an SSH connection for 10 minutes after the build finishes running and then automatically shuts down. After you SSH into the build, the connection will remain open for two hours.
