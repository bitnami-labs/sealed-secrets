# Proposal process

The purpose of a proposal is to build consensus on a problem statement and solution design before starting work on the implementation.

- A proposal is a design document that describes a **significant** change to Kubeapps.
- A proposal must be **sponsored** (or co-authored) by at least one maintainer.
- Proposals can be submitted and reviewed by **anyone** in the community.

## When to submit a proposal

If there is significant risk with a potential feature or track of work (such as complexity, cost to implement, product viability, etc.), then we recommend creating a proposal for feedback and approval.

If a potential feature is well understood and doesn't impose risk, then we recommend a standard [GitHub issue](https://github.com/vmware-tanzu/kubeapps/issues/new?assignees=&labels=&template=issue.md&title=) to clarify the details.

If you are considering creating a PR to change Kubeapps's source code, and you are not sure if the change is significant enough to require using the proposal process, then please ask the maintainers.

### When to submit an issue

If you would like to simply share a problem that you are having, or share an idea for a potential feature, and you are not planning on designing a technical solution or submitting an implementation PR, then please feel free to create a standard [GitHub issue](https://github.com/vmware-tanzu/kubeapps/issues/new?assignees=&labels=&template=issue.md&title=) instead of using the proposal process.

## How to submit a proposal

1. Open a new GitHub issue in this repo and choose the ["Proposal request"](https://github.com/vmware-tanzu/kubeapps/issues/new?assignees=&labels=kind%2Fproposal&template=proposal-request.md&title=) issue template.
2. After creating the proposal, note the issue's number. This issue can be used as a place for conversations beyond/between the proposal and implementation PRs.
3. The proposal should be documented as a separate markdown file pushed to the root of the [proposals](./) folder in the [Kubeapps repository](https://github.com/vmware-tanzu/kubeapps) via PR.

## Proposal States

To track the proposal the following states are defined:

| Status        | Definition                                                                        |
| ------------- | --------------------------------------------------------------------------------- |
| `draft`       | The proposal is actively being written by the proposer. Not yet ready for review. |
| `in-review`   | The proposal is being reviewed by the community and the project maintainers.      |
| `accepted`    | The proposal has been accepted by the project maintainers.                        |
| `rejected`    | The proposal has been rejected by the project maintainers.                        |
| `implemented` | The proposal was accepted and has since been implemented.                         |

## Proposal Lifecycle

1. Author creates a ["Proposal request"](https://github.com/vmware-tanzu/kubeapps/issues/new?assignees=&labels=kind%2Fproposal&template=proposal-request.md&title=).
2. Author adds the proposal by creating a PR in draft mode (authors can save their work until ready):
   - this PR must include a markdown file in the root of the [proposals](./) folder, and
   - this PR must reference the "Proposal request" (by adding the GitHub ID).
3. When the author elaborates the proposal sufficiently and considers ready to be reviewed:
   - change the status of the "Proposal request" to `in-review`, and
   - mark the PR as "Ready for Review".
4. The community reviews the proposal by adding PR reviews in order to mature/converge on the proposal.
5. When the maintainers reach consensus or supermajority to accept a proposal, they:
   - change the status of the "Proposal request" to `accepted`,
   - add the Engineering Decision Record (EDR) to the proposal (in the markdown file) including the following topics:
     1. _considered options_,
     2. _pros and cons_,
     3. _decision drivers_, and
     4. _decision outcome._
   - merge the PR, thus adding the new proposal to the `main` branch,
   - code implementation PRs are submitted separately to implement the solution.
6. During the implementation of an accepted proposal:
   - as each implementation PR is created, the "Proposal request" should be updated to link to the new implementation PR, and
   - when all the implementation PRs are merged, the "Proposal Request" should be updated to the `implemented` status, to list all the related PRs, and then, the "Proposal request" should be closed.
   - The proposal file (.md file) should be updated by including the version where the proposal was released.
   - If it is discovered that significant unanticipated changes are needed to the proposal, then the implementation work should be paused and the proposal should be updated with the new details to be reviewed by the maintainers again before resuming implementation.
7. When the maintainers do not reach consensus or supermajority, then the proposal is rejected, and they:
   - may mark the status of the "Proposal request" as `rejected`,
   - close the PR with a note explaining the rejection, and
   - close the "Proposal request".
8. Rejected proposal PRs (and the corresponding "Proposal request") may be reopened and moved back to `in-review` if there are material changes to the proposal which address the reasons for rejection.

## Proposal Review

Once a proposal PR is marked as "Ready for Review", the community and all Kubeapps maintainers shall review the proposal. The goal of the review is to gain an understanding of the problem being solved and the design of the proposed solution.

Maintainers will consider all aspects of the proposed problem and solution, including but not limited to:

- Is the problem within the scope of the project?
- Would the additional future cost of maintenance imposed by an implementation of the solution justify solving the problem?
- Is the solution reasonably consistent with the rest of the project?
- How does the solution impact the usability, security, scalability, performance, observability, and reliability of Kubeapps?
- How might an implementation of the solution be architected and tested via automation?
- What risks might be introduced by an implementation of the solution?
- The opportunity cost of the time it would take to implement the solution, if the implementation is to be done by the maintainers.

## Maintenance of Accepted Proposal Documents

Proposal documents reflect a point-in-time design and decision. Once approved, they become historical documents, not living documents. There is no expectation that they will be maintained in the future. Instead, significant changes to a feature that came from a previous proposal should be proposed as a fresh proposal. New proposals should link to
previous proposals for historical context when appropriate.

## Getting Help with the Proposal Process

Please reach out to the maintainers in the Kubernetes Slack Workspace within
the [#kubeapps](https://kubernetes.slack.com/messages/kubeapps) channel with any questions.
