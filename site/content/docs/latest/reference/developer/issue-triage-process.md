# 🔬 Triaging and prioritizing issues

This document captures how the Kubeapps team triages and prioritizes issues.

## 📝 Issues

To guide and simplify the process, Kubeapps provides a set of issue templates to create new issues:

| Template                                                                                                                                    | Description                                       |
| ------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------- |
| [Bug report](https://github.com/vmware-tanzu/kubeapps/issues/new?assignees=&labels=kind%2Fbug&template=bug-report.md&title=)                | File a bug for something not working as expected. |
| [Feature request](https://github.com/vmware-tanzu/kubeapps/issues/new?assignees=&labels=kind%2Fproposal&template=feature-request.md&title=) | File a request for a new feature.                 |
| [Support request](https://github.com/vmware-tanzu/kubeapps/issues/new?assignees=&labels=kind%2Fquestion&template=support-request.md&title=) | File a request for questions and support.         |
| [Issue](https://github.com/vmware-tanzu/kubeapps/issues/new?assignees=&labels=&template=issue.md&title=)                                    | File an issue to describe work to be performed.   |

Kubeapps keeps a backlog of issues on GitHub submitted both by maintainers and contributors.

- The backlog is completely open. The maintainer team, along with the entire Kubeapps community, files all feature enhancements, bugs, and potential future work into the open repository.
- Issues are closed if solved, or outdated (meaning the issue does not apply anymore according to the evolution of Kubeapps or it was waiting for more information which was never received).

## 🏷 Labeling

Kubeapps maintainers team operates with 4 groups of labels:

### `kind/*`:

The type of issue. kind powers our filtering to understand what qualifies as a bug, proposal, feature, enhancement, question or documentation.

| Label                | Description                                                      |
| -------------------- | ---------------------------------------------------------------- |
| `kind/bug`           | An issue that reports a defect in an existing feature            |
| `kind/documentation` | An issue that reports an update related to project documentation |
| `kind/proposal`      | An issue that reports a new feature proposal to be discussed     |
| `kind/feature`       | An issue that reports a feature (approved) to be implemented     |
| `kind/enhancement`   | An issue that reports an enhancement for an implemented feature  |

### `component/*`:

The relevant component(s) for the issue. Components are high level areas of the Kubeapps architecture. They are used to group issues together with other related issues.

| Label                        | Description                                                                                       |
| ---------------------------- | ------------------------------------------------------------------------------------------------- |
| `component/api-server`       | An issue related to kubeapps api-server                                                           |
| `component/apprepository`    | An issue related to kubeapps apprepository                                                        |
| `component/asset-syncer`     | An issue related to kubeapps asset-syncer (to be deprecated)                                      |
| `component/assetsvc`         | An issue related to kubeapps assetsvc (to be deprecated)                                          |
| `component/authentication`   | An issue related to kubeapps authentication                                                       |
| `component/ci`               | An issue related to kubeapps ci system                                                            |
| `component/kubeops`          | An issue related to kubeops (to be deprecated)                                                    |
| `component/packages`         | An issue related to kubeapps packaging formats to be distributed (Helm chart and Carvel packages) |
| `component/pinniped-proxy`   | An issue related to kubeapps integration with pinniped-proxy                                      |
| `component/plugin-carvel`    | An issue related to kubeapps plugin to manage Carvel packages                                     |
| `component/plugin-flux`      | An issue related to kubeapps plugin to manage Flux packages                                       |
| `component/plugin-helm`      | An issue related to kubeapps plugin to manage Helm packages                                       |
| `component/plugin-operators` | An issue related to kubeapps plugin to manage operators (to be implemented)                       |
| `component/plugin-resources` | An issue related to kubeapps plugin to manage resources                                           |
| `component/ui`               | An issue related to kubeapps UI                                                                   |

### `contribution labels`:

Specific labels for contributors. Contribution labels help to identify a relevant attribute of the issue.

| Label                    | Description                                                                      |
| ------------------------ | -------------------------------------------------------------------------------- |
| `awaiting-more-evidence` | Need more info requested to actually get it done.                                |
| `cla-notrequired`        | Automatic label for CLA signature                                                |
| `cla-rejected`           | Automatic label for CLA signature when rejected                                  |
| `dependencies`           | Automatic label set to pull requests that update a dependency file               |
| `github_actions`         | Label assigned to pull requests that update GitHub Actions code                  |
| `go`                     | Automatic label set to pull requests that update Go code                         |
| `good first issue`       | Good first issues to start contributing to Kubeapps.                             |
| `help wanted`            | The maintainer team wants help on an issue or pull request.                      |
| `javascript`             | Automatic label set to pull requests that update javascript code                 |
| `next-iteration`         | Label to mark issues to be discussed in the next planning session                |
| `rust`                   | Automatic label set to pull requests that update rust code                       |
| `security`               | Issues which relate to security concerns.                                        |
| `stale`                  | Automatic label to stale issues due inactivity to be closed if no further action |
| `wontfix`                | Issue marked by the maintainers team as not fixable                              |

### Metadata

There is some metadata for Kubeapps project in GitHub to be added to the issues:

| size/ |                                                                                             |
| ----- | ------------------------------------------------------------------------------------------- |
| 'XS'  | A task that can be done by a person in less than 1 full day                                 |
| 'S'   | A story that can be done by a person in 1-3 days, with no uncertainty                       |
| 'M'   | A story that can be done by a person in 4-7 days, possibly with some uncertainty            |
| 'L'   | A story that requires investigation and possibly will take a person a full 2-week iteration |
| 'XL'  | A story too big or with too many unknowns. Needs investigation and split into several ones  |

| priority/ | Description                                  |
| --------- | -------------------------------------------- |
| ⛔️ P0    | Unbreak-now. Drop everything and fix it      |
| 🔴 P1     | Required to be done before other things      |
| 🟠 P2     | Ordinary flow of work                        |
| 🔵 P3     | Nice to have, but not required to be tackled |

## ⛳️ Milestones

[Milestones](https://github.com/vmware-tanzu/kubeapps/milestones) are used by Kubeapps maintainers to map issues into EPICs. An EPIC represents a series of issues that share a broader strategic objective. An EPIC will typically require development work covering several iterations (in our case, EPICs must be defined to be covered in a quarter). Every triaged issue should have a milestone associated.

Kubeapps EPICs must include: `title`,`description`, `acceptance criteria`, `end-date` (the end of a quarter).

## ❔ Triaging process

[Kubeapps](https://github.com/vmware-tanzu/kubeapps) new issues will be triaged by the maintainers team. The triage process will consist of:

- **At any moment**:
  - Read the new issue:
    - If more information is requested, the issue must be labeled as `awaiting-more-evidence` and a comment requesting for information should be added.
    - Metadata for size and priority for the issue must be set as: `Needs triage`.
    - Check if it is an issue to be accomplished as soon as possible (`P0`) and move it to the **To Do** column.
    - If not, the issue must remain in the **Backlog** column.
- **During planning session**:
  - Review issues marked as `Needs triage`.
  - Add labels and update metadata (required): `kind`, `component`, `size` and `priority`.
    - Check if it is a `good-first-issue` to start contributing to Kubeapps and label the issue as such.
    - Check if it is an issue to be discussed to be included in next iteration and label it as `next-iteration`.

### 🗄 Stale issues

Automatically **stalebot** is checking inactive issues to label them as `stale`. An issue becomes stale after 15 days of inactivity and the bot will close it after 3 days of inactivity for stale issues. To be considered:

- Issues labeled as `kind/feature`, `kind/bug` or `kind/refactor` will never be labeled as `stale`.
- Only issues labeled as `awaiting-more-evidence` could be considered stale.
- The label to use when marking an issue as stale is `stale`.

## 🔢 Prioritizing process

This process mainly consists on checking issues in the **Backlog** and moving to the **To Do** column to be tackled during the following iteration.

Criteria:

1. Issues marked as `P0` → Add to the iteration (**To Do**).
2. Review issues to complete those milestones planned for the current quarter and select those to be completed during next iteration by priority. To be considered for the capacity:

- At least 1 issue labeled as `kind/bug` must be included for every single iteration.
- At least 1 issue **requested** from the Kubeapps community must be included for every single iteration.

3. Then review issues labeled as `next-iteration` and discuss what issues should be included according to the maintainers team capacity, issues already added to the **To Do** column and uncompleted issues from previous iterations (**In progress**).
