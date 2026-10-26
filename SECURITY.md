# Security policy

Amaru is at the foundation stage: this repository holds documentation, decisions and a documentation checker. There is no released software to patch yet. This policy is in place for the first public release and applies to everything in this repository ([security model](docs/security/SECURITY_MODEL.md)). Approved by the product owner on 2026-10-26 ([#71](https://github.com/rluders/amaru/issues/71)).

## Reporting a vulnerability
Do not open a public issue or pull request for a vulnerability.

Report it privately through GitHub: **Security tab → "Report a vulnerability"** on this repository (GitHub Security Advisories). Include the affected component and version or commit, steps to reproduce, the impact you see, and whether you have shared it with anyone else.

Status of the channel: private vulnerability reporting is a repository setting that the owner enables when the repository becomes public; while the repository is private there is no external reporting channel and no external reports are expected. No e-mail address is published.

## What to expect
These are targets, not guarantees; Amaru has a single maintainer.
1. **Acknowledgement** within 7 days of the report.
2. **Assessment** and an initial severity and scope decision within 30 days.
3. **Fix and advisory**: coordinated disclosure within **90 days** of the report, as decided in the security model. A fix may ship sooner; the window may be extended by agreement with the reporter if a fix needs more time.
4. **Credit** in the advisory if you want it.

We will not pursue action against good-faith research that avoids privacy violations, data destruction and service disruption, and reports promptly. Do not access data that is not yours.

## Supported versions
None yet. Once releases exist, this section will list the versions that receive security fixes.

## Out of scope
Findings that need an already-compromised host or a malicious local administrator are handled by the [security model](docs/security/SECURITY_MODEL.md) residual-risk list, not as vulnerabilities, unless they cross a documented boundary.
