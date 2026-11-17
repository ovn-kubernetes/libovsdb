# Contributing Guide

* [Ways to Contribute](#ways-to-contribute)
* [Find an Issue](#find-an-issue)
* [Ask for Help](#ask-for-help)
* [Pull Request Lifecycle](#pull-request-lifecycle)
* [Development Environment Setup](#development-environment-setup)
* [Sign Your Commits](#sign-your-commits)
* [Logical Grouping of Commits](#logical-grouping-of-commits)
* [Commit Message Guidelines](#commit-message-guidelines)
* [Pull Request Checklist](#pull-request-checklist)
* [AI Guidelines](#ai-guidelines)

Welcome! We are glad that you want to contribute to libovsdb!

All contributors are expected to follow our [Code of Conduct](./CODE_OF_CONDUCT.md).

As you get started, you are in the best position to give us feedback on areas of
the project that we need help with including:

* Problems found during setting up a new developer environment
* Gaps in documentation
* Bugs in our automation scripts

If anything doesn't make sense, or doesn't work when you run it, please open a
bug report and let us know!

## Ways to Contribute

We welcome many different types of contributions including:

* New features
* Bug fixes
* Builds, CI/CD
* Documentation
* Issue triage
* Answering questions on Slack
* Participating in community meetings

### Come to Meetings

Everyone is welcome to join our meetings; you do not need an invitation or a
prepared contribution. libovsdb currently shares the wider ovn-kubernetes
community meeting. See [MEETINGS.md](./MEETINGS.md) for details.

## Find an Issue

We have [good first issues](https://github.com/ovn-kubernetes/libovsdb/labels/good%20first%20issue)
for new contributors and [help wanted](https://github.com/ovn-kubernetes/libovsdb/labels/help%20wanted)
issues suitable for any contributor.

Sometimes there will not be any issues with these labels. If you want to
contribute but cannot find a suitable issue, ask in the [#libovsdb
channel](https://cloud-native.slack.com/archives/C083G8GF7GA) on the CNCF Slack server and we will
be happy to help.

Once you see an issue that you'd like to work on, please post a comment saying
that you want to work on it. Something as simple as "I want to work on this" is
fine.

## Ask for Help

The best way to reach us with a question when contributing is to ask on:

* The original GitHub issue
* The [#libovsdb channel](https://cloud-native.slack.com/archives/C083G8GF7GA) on the CNCF Slack server

## Pull Request Lifecycle

1. Fork the repository and open a PR against `main`
2. Make sure your PR is passing CI — if you need help with failing checks please ask!
3. Once all CI checks pass, a maintainer will review your PR and may request changes
4. When you have received at least one approval from a maintainer, it will be merged

If your PR conflicts with other changes, you will be asked to rebase.

## Development Environment Setup

### Prerequisites

Install [mise](https://mise.jdx.dev/getting-started.html), a polyglot tool version
manager. mise reads `mise.toml` in the repository root and installs the exact
versions of all development tools (Go, golangci-lint, benchstat) declared there.

```console
# Install mise (see https://mise.jdx.dev/getting-started.html for alternatives)
curl https://mise.run | sh
```

### Getting Started

```console
git clone https://github.com/ovn-kubernetes/libovsdb.git
cd libovsdb

# Install all declared tool versions
mise install
```

After `mise install`, the correct versions of `go`, `golangci-lint`, and
`benchstat` are available in your shell (mise shims them automatically).

If you add or change tools, update `mise.toml` and commit it so everyone stays
in sync. CI uses the same file via `jdx/mise-action`.

### Building

```console
make build
```

### Running Unit Tests

```console
make test
```

### Running Integration Tests

Integration tests require a running OVS instance. The easiest way is via Docker:

```console
make integration-test
```

You can target a specific OVS version with:

```console
OVS_VERSION=v3.4.0 make integration-test
```

### Linting

```console
make lint
```

### Benchmarks

```console
make bench
```

## Sign Your Commits

### DCO

We require that contributors sign off on commits submitted to the project. The
[Developer Certificate of Origin (DCO)](https://developercertificate.org/) is a
way to certify that you wrote and have the right to contribute the code you are
submitting.

You sign off by adding the following to your commit messages. Your sign-off must
match the git user and email associated with the commit.

```
Signed-off-by: Your Name <your.name@example.com>
```

Git has a `-s` flag to do this automatically:

```console
git commit -s -m 'your commit message'
```

If you forgot to sign off and have not yet pushed, amend the commit:

```console
git commit --amend -s
```

## Logical Grouping of Commits

Keep changes logically grouped into individual commits. If you use checkpoint
commits while developing, squash them into coherent changes before asking for
review.

When addressing review comments, prefer fixing up the relevant commit rather
than adding commits such as `fix review comments`. If you need to both rebase
and revise commits, push those operations separately when practical so GitHub's
comparison links remain useful to reviewers.

## Commit Message Guidelines

A good commit message should describe what changed and why.

1. The first line should:
   * contain a short description of the change (50 characters or less, no more than 72)
   * be entirely in lowercase except for proper nouns, acronyms, and code references
   * be prefixed with the name of the sub-component being changed

   Examples:
   * `client: fix reconnect panic on concurrent disconnect`
   * `server: add support for conditional updates`
   * `model: validate schema on load`

2. Keep the second line blank.
3. Wrap all other lines at 72 columns (except for long URLs).
4. Reference issues where relevant. Use `Fixes:` when the change closes an
   issue and `Refs:` for related issues. Multiple references may be separated
   by commas:
   * `Fixes: #1337`
   * `Refs: #1234, #5678`

Example:

```
client: fix reconnect panic on concurrent disconnect

When Disconnect is called while a reconnect is in progress, the
connection manager now waits for the in-flight attempt to complete
before tearing down state, avoiding a nil pointer dereference.

Fixes: #456
```

## Pull Request Checklist

Before submitting your pull request:

* [ ] Code is formatted: `gofmt -l .` returns no output
* [ ] Linter passes: `make lint`
* [ ] Unit tests pass: `make test`
* [ ] New behaviour is covered by tests
* [ ] User-facing changes include documentation updates
* [ ] Commits are logically grouped and have clear commit messages
* [ ] Commits are signed off (`git commit -s`)

## AI Guidelines

You may use AI tools when preparing contributions such as issues and pull
requests. As the author, you remain responsible for understanding every change
you submit.

* Review and test AI-generated changes yourself before submitting. Do not rely
  on reviewers to perform the first review.
* Be prepared to explain any change in your pull request. If you cannot explain
  why a change was made, the pull request may be closed.
* Respond to review comments in your own words. Reviewers want to engage
  directly with you, not with generated responses.
* Disclose AI tool use when it materially contributed to your submission, as
  described below.
* All contributions must follow this guide. Large AI-generated pull requests
  and AI-generated commit messages are discouraged.

### Disclosure

Disclose AI tool use when it materially contributed to what you submit. You
must still adhere to the [DCO](#dco) and sign off your commits.

Use the "Special notes for your reviewer" section in the pull request, and
optionally add a commit trailer. Acceptable trailer formats include:

* `Assisted-by: Tool Name and/or Model Name <contact@example.com>`
* `Co-authored-by: Tool Name and/or Model Name <contact@example.com>`

Many AI tools add `Co-authored-by` automatically; that is acceptable.

Disclosure is required when AI wrote or suggested code, tests, documentation,
or commit messages that you used, or suggested an approach or refactor that
shaped the implementation.

Disclosure is not required for general questions or learning, IDE autocomplete,
using AI to explain or review existing code, spell-checking, or minor fixes.
When in doubt, disclose.
