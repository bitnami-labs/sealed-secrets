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

A release train is created by *branching* from `main` to `release/YYYYMMDD`, where `YYYYMMDD` is a ISO 8601 numbered timestamp of the release day date.

#### Validation

The `release/...` branch will go through the release CI. If anything fails the release branch is dropped, the issue fixed in `main` and a new release train is started on a new branch.

#### Tagging

Once the release passes all validations and is published, it is merged into `released`. Then, it is tagged with the final version, following SemVer semantics as `vX.Y.Z`.

#### Hot-fixing releases

If there is a need to urgently fix a show-stopper issue in the latest released version. A fix can be worked on right away in a `hotfix` branch directly off `released`.

Once the fix is merged, the resulting `released` branch is manually tagged with the new patch release `vX.Y.Z` and that tag is published.

After that, *the hotfix is back-ported to the `main` branch including the tests added to detected regressions* of that issue going forward.
