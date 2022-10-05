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

### PR Approval and Release Process

1. Changes are manually reviewed by VMware/Bitnami team members.
2. When the PR passes all tests, the PR is merged by the reviewer(s) in the GitHub `main` branch.
