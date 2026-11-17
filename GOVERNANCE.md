# libovsdb Project Governance

libovsdb is an official subproject of
[ovn-kubernetes](https://github.com/ovn-kubernetes/ovn-kubernetes). It provides a
Go library for interacting with Open vSwitch Database (OVSDB) servers.

This document describes how the libovsdb subproject is governed. libovsdb also
follows the [ovn-kubernetes project governance](https://github.com/ovn-kubernetes/ovn-kubernetes/blob/master/GOVERNANCE.md).
Where this document is silent or conflicts with the parent project's
governance, the ovn-kubernetes governance takes precedence.

- [Values](#values)
- [Maintainers](#maintainers)
  - [Responsibilities](#responsibilities)
  - [Becoming a Maintainer](#becoming-a-maintainer)
  - [Removing a Maintainer](#removing-a-maintainer)
- [Decision Making](#decision-making)
- [Repository Access](#repository-access)
- [Meetings and Communication](#meetings-and-communication)
- [Code of Conduct](#code-of-conduct)
- [Security](#security)
- [Modifying this Governance](#modifying-this-governance)

## Values

libovsdb and its leadership embrace the values of the wider ovn-kubernetes
project:

- **Openness:** Communication and decision making happen in the open and remain
  discoverable for future reference whenever possible.
- **Fairness:** Contributions and feedback are considered on their merits.
- **Community over company:** Sustaining a healthy project community takes
  priority over the interests of any single organization.
- **Inclusive participation:** Contributors are welcomed regardless of
  background, employer, identity, or level of experience.
- **Technical excellence:** Changes should preserve the reliability,
  compatibility, and maintainability expected of a broadly used library.

## Maintainers

Maintainers are the people responsible for the day-to-day stewardship of the
libovsdb subproject. The current Maintainers and project alumni are listed in
[`MAINTAINERS`](./MAINTAINERS).

Maintainer status reflects sustained responsibility for the project; it is not
merely permission to merge changes. Maintainers are expected to collaborate,
seek appropriate review, and act in the best interests of libovsdb and the
wider ovn-kubernetes community.

### Responsibilities

Maintainers collectively:

- review and merge code, documentation, automation, and dependency changes;
- maintain the quality, compatibility, security, and release processes of the
  library;
- help contributors participate effectively and ensure reviews make progress;
- maintain project infrastructure and repository settings;
- represent libovsdb in the wider ovn-kubernetes community;
- document significant technical and governance decisions; and
- enforce the project's [Code of Conduct](./CODE_OF_CONDUCT.md).

Maintainers may merge their own changes when they have received appropriate
review. Substantial, risky, or user-visible changes should be reviewed by
another Maintainer or an appropriate subject-matter expert.

### Becoming a Maintainer

Prospective Maintainers should already be demonstrating Maintainer-level
responsibility. Relevant evidence includes:

- sustained, high-quality contributions to code, tests, documentation, reviews,
  or project operations;
- a strong understanding of libovsdb's architecture, compatibility guarantees,
  development process, and review standards;
- constructive collaboration with contributors and the ovn-kubernetes
  community;
- reliable follow-through on reviews, issues, regressions, and releases; and
- sound judgment when balancing technical quality, compatibility, and community
  needs.

An existing Maintainer may nominate a candidate after confirming that the
candidate is willing to serve. The nomination should summarize the candidate's
contributions and may be discussed privately when personal considerations are
involved. Appointment requires a two-thirds vote of the current Maintainers.
The result is recorded by updating [`MAINTAINERS`](./MAINTAINERS).

### Removing a Maintainer

Maintainers may resign at any time. A Maintainer may also be removed for
prolonged inactivity, failure to fulfill Maintainer responsibilities, a Code of
Conduct violation, or other circumstances that make continued service
inappropriate.

Inactivity generally means a year or more of very low or no project activity
without a known plan to return. Removal requires a two-thirds vote of the other
Maintainers. Depending on the circumstances, the Maintainer may be moved to the
Alumni list and may later be reinstated through the normal appointment process.

## Decision Making

Most project decisions are made through
[open discussion and lazy consensus](https://community.apache.org/committers/lazyConsensus.html)
on issues and pull requests. Significant decisions should allow reasonable time
for affected contributors and the wider ovn-kubernetes community to respond.

Any Maintainer may request a formal vote when consensus cannot be reached. Unless
this document states otherwise, a vote requires a simple majority of all current
Maintainers. A tie means that the proposed change does not proceed. Security,
Code of Conduct, and personnel matters may be discussed and voted on privately.

Decisions that materially affect the parent ovn-kubernetes project should be
coordinated with its Maintainers and follow its governance and enhancement
processes where applicable.

## Repository Access

Repository permissions are granted according to project responsibilities and
the principle of least privilege. Maintainers receive the access needed to
review and merge changes and operate the project. Administrative access should
be limited to Maintainers who need it for repository or organization
management.

Access must be updated when a person's project role changes. GitHub permissions
do not override the review, voting, or conduct requirements in this document.

## Meetings and Communication

libovsdb shares the public ovn-kubernetes community meeting. Details are in
[`MEETINGS.md`](./MEETINGS.md). Everyone is welcome to attend.

Day-to-day discussion also takes place in the dedicated
[#libovsdb channel](https://cloud-native.slack.com/archives/C083G8GF7GA) on CNCF
Slack and in GitHub issues and pull requests. Decisions made in meetings or
Slack that affect the project should be recorded in a durable public location,
such as an issue or pull request.

Closed discussions may be used for security reports, Code of Conduct reports,
and sensitive personnel matters. Participation must be limited to the people
needed to handle the matter, and anyone with a conflict of interest must recuse
themselves.

## Code of Conduct

All project participants must follow the
[libovsdb Code of Conduct](./CODE_OF_CONDUCT.md) and the conduct requirements of
the wider ovn-kubernetes and CNCF communities.

Reports are handled privately and must protect the privacy and security of the
reporter. A Maintainer who is the subject of a report must not participate in
handling that report.

## Security

Security reports are handled in coordination with the ovn-kubernetes project
and its [security policy](https://github.com/ovn-kubernetes/ovn-kubernetes/blob/master/SECURITY.md).
Potential vulnerabilities must not be discussed in public issues or public
Slack channels before coordinated disclosure.

The Maintainers are responsible for ensuring that security reports are handled
promptly. They may delegate a report to an appropriate response team, including
ovn-kubernetes Maintainers or subject-matter experts, while limiting disclosure
to those who need access.

## Modifying this Governance

Changes to this document require a two-thirds vote of the current libovsdb
Maintainers. Changes must remain consistent with the ovn-kubernetes project
governance; changes that affect the relationship with the parent project must
also be coordinated with ovn-kubernetes Maintainers.
