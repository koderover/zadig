# Contributing Guidelines

First, thank you for considering contributions to Zadig!

We welcome all sorts of contributions: from fixing a typo, updating a documentation, raising a bug, proposing a feature request, to proposing a design and driving all the implementations in end-to-end manner.

## TOC

- [Contributing Guidelines](#contributing-guidelines)
  - [TOC](#toc)
  - [Prerequisite](#prerequisite)
  - [Setup](#setup)
  - [Contribution option 1 - reporting an issue](#contribution-option-1---reporting-an-issue)
    - [How issues will be handled](#how-issues-will-be-handled)
  - [Contribution option 2 - update a doc](#contribution-option-2---update-a-doc)
    - [For trivial doc changes](#for-trivial-doc-changes)
    - [For non-trivial doc changes](#for-non-trivial-doc-changes)
  - [Contribution option 3 - code changes](#contribution-option-3---code-changes)
    - [For trivial changes](#for-trivial-changes)
    - [For non-trivial changes](#for-non-trivial-changes)
  - [Contributor resources](#contributor-resources)
    - [PR and commit guidelines](#pr-and-commit-guidelines)
    - [Growth path for contributors](#growth-path-for-contributors)
    - [Where can you get support](#where-can-you-get-support)
    - [More resources](#more-resources)

## Prerequisite

For every commit, **be sure to sign off on the [DCO](https://github.com/probot/dco#how-it-works).**

Zadig's success heavily relies on a healthy community, and it's up to every one of us to maintain it. This is a
reminder that all contributions are expected to adhere to our [Code of Conduct](CODE_OF_CONDUCT.md).

## Setup

You should first **fork the specific repository** you want to contribute to. Please follow the instructions
[here](community/dev/contributor-workflow.md) to set Zadig up for running.

Before submitting any new issues or proposing any new changes, please check the
[open issues](https://github.com/koderover/Zadig/issues) to make sure the efforts aren't duplicated. There are
a few labels that could help you navigate through the issues:

- If you are looking for some startup issues to get your hands on, you could follow the
  [#good-first-issue](https://github.com/koderover/Zadig/labels/good%20first%20issue) issues.

- If you are looking for some more serious challenges, you could check out our
  [#help-wanted](https://github.com/koderover/Zadig/labels/help%20wanted) issues.

- Or some random bugfix, check out [#bugs](https://github.com/koderover/Zadig/labels/bug).

## Contribution option 1 - reporting an issue

**For security related issues, please drop an email to contact@koderover.com instead.** For non-security related
issues, please read on.

There are 5 types of labels for issues that you need to think of:

1. [`documentation`](https://github.com/koderover/Zadig/labels/documentation)
2. [`bug`](https://github.com/koderover/Zadig/labels/bug)
3. [`feature request`](https://github.com/koderover/Zadig/labels/feature%20request)
4. [`question`](https://github.com/koderover/Zadig/labels/question)
5. [`enhancement`](https://github.com/koderover/Zadig/labels/enhancement)

If you understand which services are involved with the new issue, please also attach the relevant label(s) to it. You
could [search for `service/` prefix](https://github.com/koderover/Zadig/labels?q=service%2F) to find the correct label.
If you aren't sure about this, feel free to leave it open, our maintainers will get to it.

If you have checked that no duplicated issues existed and decided to create a new issue, choose the label accordingly
and follow the issue template. Please be as specific as possible assuming low or no context from whoever reading the
issue.

### How issues will be handled

All issues created will be triaged by our maintainers:

1. The maintainers will double check the issues were created with the proper label, update them otherwise.
2. They'll make decisions whether the issues will be accepted, see next point.
3. Certain tags might be applied by our maintainers accordingly, there are mainly 4 of them:
   1) [`duplicate`](https://github.com/koderover/Zadig/labels/duplicate)
   2) [`wonfix`](https://github.com/koderover/Zadig/labels/wontfix):
      rationales will be provided, e.g. work as intended, obsolete, infeasible, out of scope
   3) [`good first issue`](https://github.com/koderover/Zadig/labels/good%20first%20issue): good candidates
      suitable for onboarding.
   4) [`good intermediate issue`](https://github.com/koderover/Zadig/labels/good%20intermediate%20issue): a more
      serious challenge that is up for grabs to the community.
4. The issues are open for grab now.
5. Periodically, the maintainers will double check that all issues are up-to-date, stale ones will be removed.

## Contribution option 2 - update a doc

### For trivial doc changes

For trivial doc changes, such as fixing a typo, updating a link, go ahead and create a pull request. One of our
maintainers will get to it. Please follow our [PR and commit guidelines](#pr-and-commit-guidelines) below.

### For non-trivial doc changes

If you are looking for a more serious documentation changes, such as refactoring a full page, please follow the
[instructions of non-trivial code changes](#for-non-trivial-changes) below. You'll need an issue to track this and a
design outlining your proposal and have the design approved first.

## Contribution option 3 - code changes

For any code changes, you **need to have an associated issue**: either an existing one or
[create a new one](#contribution-option-1---reporting-an-issue).

> to prevent duplication, please claim an issue by commenting on it that you are working on the issue.

### For trivial changes

1. You could describe your design ideas in the issue; but if you are confident enough with your
   change, feel free to skip this step too.
2. Work on the changes in your forked repository.
3. Please follow our [PR and commit guidelines](#pr-and-commit-guidelines), one of our maintainers will review your PR.

### For non-trivial changes

1. You need to write a design to describe what you are trying to address and how by making a copy of
   [Zadig design template](community/rfc/yyyy-MM-dd-my-design-template.md) and filling in the template.
2. Add the design in markdown format in [community/rfc folder](community/rfc), name it as
   `yyyy-MM-dd-my-design-name.md`. Please send a separate PR for this. One of our maintainers will get to it.
3. Once the design is approved and PR merged, you can start working on the issue following the solutions outlined in
   the design.
4. We highly encourage atomic PRs, so if your change can be broken down into several smaller sub-tasks, please do it
   and create one PR for each.
5. For each sub-task,  follow the [trivial-changes guidelines above](#for-trivial-changes).

## Contributor resources

### PR and commit guidelines

- Please follow our PR template and write clear descriptions as much as possible.
- Please keep each commit atomic and write crisp, clear and accurate commit messages.
- Don't forget signing off the [DCO](https://github.com/probot/dco#how-it-works) for each commit.
- We recommend you to create PR **early** once you decide to work on something. However, it must be prefaced with
  "WIP: [title]" until it's ready for review. Once it is ready for review, remove the prefix "WIP: ", one of our
  maintainers will get to it.
- Make sure your changes passed all the tests.
- Please rely on the official [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) as the
  code style guideline.
- Attach the relevant label(s) to your PR by following the same guideline for issues in [Contribution option 1 - reporting an issue](#contribution-option-1---reporting-an-issue)
- To test your changes, please follow [how to debug your code](community/dev/contributor-workflow.md#4-调试)

### Growth path for contributors

We have established a well-lit path for contributor's progression. Please check out [GOVERNANCE](GOVERNANCE.md) for
more information.

### Where can you get support

- Email：contact@koderover.com
- [slack channel](https://join.slack.com/t/zadig-workspace/shared_invite/zt-qedvct1t-mQUf2eyTRkoVCc_RWKKgxw)

### More resources

- [Current maintainers](GOVERNANCE.md#maintainers)