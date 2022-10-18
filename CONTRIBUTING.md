# Contributing Guidelines

Contributions are welcome via GitHub Pull Requests. This document outlines the process to help get your contribution accepted.

Any type of contribution is welcome; from new features, bug fixes, or documentation improvements. However, VMware/Bitnami will review the proposals and perform a triage over them. By doing so, we will ensure that the most valuable contributions for the community will be implemented in due time.

## How to Contribute

1. Fork this repository, develop, and test your changes.
2. Submit a pull request.

### Technical Requirements

When submitting a PR make sure that it:

- Must pass CI jobs for linting and test the changes on top of different k8s platforms.
- Must follow [Golang best practices](https://go.dev/doc/effective_go).
- Is signed off with the line `Signed-off-by: <Your-Name> <Your-email>`. See [related GitHub blogpost about signing off](https://github.blog/changelog/2022-06-08-admins-can-require-sign-off-on-web-based-commits/).
  > Note: Signing off on a commit is different than signing a commit, such as with a GPG key.

### PR Approval

1. Changes are manually reviewed by VMware/Bitnami team members.
2. When the PR passes all tests, the PR is merged by the reviewer(s) in the GitHub `main` branch.

### Release process

The release process is based upon periodic release trains.

#### Schedule

Releases happen monthly. A release train "leaves" on the 15th of each month, or the closest working date to that.
 
#### Creation

First of all, prepare the release notes as usual, and merge them.

Once the release notes are ready, a release train is launched by *branching* from `main` to `release/vX.Y.Z`.

#### Validation

The `release/vX.Y.Z` branch will go through the release CI. GoReleaser requires a tag to build a release, so one will be produced automatically from the release branch name `vX.Y.Z`.

If anything fails the release branch is dropped, the issue fixed in `main` and a new release train is started on a new branch.

#### Tracking

Once the release passes all validations and is published, it is merged into `released`.

Note that currently the release process is done in 2 steps, first the container images, then the chart using them. Both events must be merged in the `released` branch.

#### Hot-fixing releases

If there is a need to urgently fix a show-stopper issue in the latest released version. There is no need to wait for the next release train for a new release to happen.

Unless there is a strong reason not to, a fix can be merged into `main` directly, followed by a regular release process.

If doing the fix in main is a "no go" for some reason, for instance, a new change already in `main` makes the bug to be urgently fixed even worse, then the fix must happen from the latest released code to proceed ASAP:

* Create a `hotfix/YYYYMMDD` branch as a copy of `released`. The `YYYYMMDD` suffix is an ISO-8601 timestamp, for tracking purposes.
* Branch off `hotfix/YYYYMMDD` to work on the fix. As a regular PR, you might name the fix branch with a descriptive name for the bug being fixed.
* Once the fix is approved and tested as successful, merge into `hotfix/YYYYMMDD`.
* Push `hostfix/YYYYMMDD` as a `release/vX.Y.Z` to kick off a release train.
* If the release fails for any reason, fix it in `hostfix/YYYYMMDD`, merge and push another `release/vX.Y.Z'` branch.
* Once a hotfix release completes successfully, merge the `release/vX.Y.Z` as `released` as per normal procedure.
* *Backport the hotfix into the `main` including the tests added to detected regressions* of that bug going forward.
* Finally, `hotfix/YYYYMMDD` can be kept around for tracking or historical purposes.

Note that, in either case, the release notes must clarify this was a hotfix our of the regular release train schedule.
