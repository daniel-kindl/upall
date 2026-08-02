# Security policy

## Reporting a vulnerability

**Do not open a public issue for a security vulnerability.**

Report it through [GitHub's private vulnerability
reporting](https://github.com/daniel-kindl/upall/security/advisories/new) — the
"Report a vulnerability" button on the Security tab. This is the preferred channel; it
is private, it threads properly, and it produces an advisory when the fix ships.

If that is not available to you, email **daniel.kindl@proton.me** with `upall
security` in the subject.

Please include:

- What the vulnerability lets an attacker do.
- Steps to reproduce it, or a proof of concept.
- The version or commit, your OS, and which providers were involved.
- Whether it is already public anywhere.

### What to expect

This project is maintained by one person working in bursts, so response times are
honest rather than aspirational:

| | |
|---|---|
| Acknowledgement | Within 7 days |
| Initial assessment | Within 14 days |
| Fix or mitigation plan | Depends on severity; you will be told which |

You will be credited in the advisory unless you would rather not be. If you get no
response within 14 days, assume the message went astray and try the other channel.

There is no bug bounty.

## Supported versions

| Version | Supported |
|---|---|
| Unreleased (pre-1.0) | Latest `main` and `dev` only |

There are no releases yet. Once 1.0 ships, this table will list supported release
lines; until then, security fixes land on `dev` and `main` and nowhere else.

## Scope

upall runs package managers, sometimes elevated, on the machine it is installed on.
That is its purpose, so some things that look alarming are the design working as
intended. The [security model in
ARCHITECTURE.md](docs/ARCHITECTURE.md#security-model) describes the posture in full.

### In scope

- Privilege escalation beyond what a provider declared it needs.
- Command injection — anywhere a value reaches a subprocess as anything other than a
  discrete argv element.
- Elevation acquired by a provider that did not declare `NeedsElevation`.
- Loading or executing a manifest from an unexpected location, or the override
  mechanism being active without the user having enabled it.
- Secrets, tokens, or environment contents leaking into logs, the run journal, or
  `--json` output.
- Anything fetched over the network by upall itself, since upall is not supposed to
  fetch anything.
- Tampering with a published release artifact or its checksum.

### Not in scope

- **Vulnerabilities in the package managers upall drives.** Report those to apt,
  winget, docker, and so on. upall passes your intent along; it does not audit them.
- **What an update does once installed.** upall installs the updates you asked for
  from the sources you already trust.
- **A user enabling manifest overrides and then running a hostile manifest.** That
  mechanism grants arbitrary command execution by design, is off by default, and says
  so where it is documented. Report it if it can be enabled *without* the user
  knowingly doing so.
- **Needing local access to exploit it.** upall is a local tool; local access is
  assumed. Local privilege *escalation* is in scope — local access alone is not.

If you are unsure whether something is in scope, report it. A wrong guess in that
direction costs nothing.
