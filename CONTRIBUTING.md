# Contributing Guide

* [Ways to Contribute](#ways-to-contribute)
* [Find an Issue](#find-an-issue)
* [Ask for Help](#ask-for-help)
* [Pull Request Lifecycle](#pull-request-lifecycle)
* [Development Environment Setup](#development-environment-setup)
* [Sign Your Commits](#sign-your-commits)
* [Commit Message Guidelines](#commit-message-guidelines)
* [Pull Request Checklist](#pull-request-checklist)

Welcome! We are glad that you want to contribute to libovsdb!

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

## Find an Issue

We have [good first issues](https://github.com/ovn-kubernetes/libovsdb/labels/good%20first%20issue)
for new contributors and [help wanted](https://github.com/ovn-kubernetes/libovsdb/labels/help%20wanted)
issues suitable for any contributor.

Once you see an issue that you'd like to work on, please post a comment saying
that you want to work on it.

## Ask for Help

The best way to reach us with a question when contributing is to ask on:

* The original GitHub issue
* The [#libovsdb channel](https://cloud-native.slack.com/) on the CNCF Slack server

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
4. Reference issues where relevant:
   * `Fixes: #1337`
   * `Refs: #1234`

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
* [ ] Commits are signed off (`git commit -s`)
