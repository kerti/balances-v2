# Security Policy

Balances is a personal-finance app that holds household net-worth data, so we take
vulnerability reports seriously. Thank you for helping keep it and its self-hosters safe.

## Reporting a vulnerability

**Please do not open a public issue for security problems.** Public issues are visible to
everyone and can expose users before a fix exists.

Instead, report privately through GitHub's **[private vulnerability
reporting](https://github.com/kerti/balances-v2/security/advisories/new)**:

1. Go to the [Security tab](https://github.com/kerti/balances-v2/security) of this repository.
2. Click **Report a vulnerability**.
3. Describe the issue, how to reproduce it, and the impact you believe it has.

This keeps the report confidential between you and the maintainer until a fix ships. If you
cannot use the GitHub form, open a regular issue asking the maintainer to reach out — **without
any vulnerability detail** — and we will follow up on a private channel.

### What to include

A good report has: affected version or commit, a description of the flaw, step-by-step
reproduction (or a proof of concept), and the impact you expect. Reports against a
self-hosted deployment should note whether the instance uses Google OAuth, local
email + password, or both (see [SELF-HOSTING.md](SELF-HOSTING.md)).

## Response expectations

Balances is maintained by a single developer, so timelines are best-effort, not contractual:

- **Acknowledgement** within **5 business days**.
- An initial **assessment** (severity, whether we can reproduce it) within **10 business days**.
- We will keep you updated as we work on a fix, and credit you in the advisory once it is
  published — unless you prefer to stay anonymous.

Please give us a reasonable window to ship a fix before any public disclosure. We aim to
coordinate a disclosure date with you.

## Supported versions

Balances is pre-`1.0.0`; the `0.x` line is an **unstable alpha/RC ramp** and makes no
compatibility promise (see
[ADR-0033](docs/adr/0033-versioning-the-upgrade-contract-and-migration-immutability.md)).
Security fixes land on `main` and go out in the **next release** rather than as backports to
older tags. Only the **latest published release** is supported — self-hosters should upgrade
to it before reporting, since the issue may already be fixed.

| Version | Supported |
|---|---|
| Latest release | ✅ |
| Any older pre-release | ❌ (upgrade to latest) |

Once a stable production release exists, this table and the backport policy will be revised
to match the operator upgrade contract in ADR-0033.

## Scope

In scope: the Balances application code in this repository — the Go backend, the frontend, the
Docker operator stack, and the migration/upgrade path.

Out of scope: vulnerabilities in third-party dependencies (report those upstream; we track
ours via Dependabot), and issues that require an already-compromised host or a
misconfiguration explicitly warned against in [SELF-HOSTING.md](SELF-HOSTING.md).
