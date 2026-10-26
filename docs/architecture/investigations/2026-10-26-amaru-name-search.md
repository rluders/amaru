# Name search for "Amaru" (preliminary, not a trademark clearance)

**Date:** 2026-10-26 · **Task:** [#70](https://github.com/rluders/amaru/issues/70), OWN-trademark, [WP-M1-01](../WORK_PACKAGES.md#wp-m1-01-license-texts-in-effect) · **Validates:** assumption A11 ([OPEN_QUESTIONS](../OPEN_QUESTIONS.md), [Q34](../OPEN_QUESTIONS.md#q34-product-name-and-trademark)) · **Run by:** an AI agent, public web and package-registry sources only.

## Result in one paragraph
**A11 is not confirmed.** No trademark register could be searched, so no registration or application for AMARU was found or ruled out. Public sources do show other software that uses the name "Amaru", including one product in the same area (developer tooling for AI coding agents). This is a decision input for the product owner, not legal advice and not a clearance. Whether the name is available is **unknown**.

## What was searched, and what could not be
| Source | Date | Outcome |
|---|---|---|
| USPTO Trademark Search (`tmsearch.uspto.gov`) | 2026-10-26 | Not usable: JavaScript application, returned no result data to a non-browser client. **Not searched.** |
| Justia Trademarks (`trademarks.justia.com`) | 2026-10-26 | HTTP 403 (bot challenge). **Not searched.** |
| EUIPO TMview (`tmdn.org/tmview`) | 2026-10-26 | Not reachable from the agent environment. **Not searched.** |
| WIPO Global Brand Database, national offices (INPI Brazil and others) | — | **Not attempted.** |
| General web search: "Amaru" trademark software developer tools | 2026-10-26 | No Amaru trademark found; results were unrelated names (not evidence of absence). |
| General web search: Amaru AI agent orchestration platform | 2026-10-26 | No product found by that query; the registry findings below show it was incomplete. |
| GitHub repository search (`gh search repos amaru`, API) | 2026-10-26 | Findings below. 631 repositories have "amaru" in the name. |
| crates.io, npm, PyPI | 2026-10-26 | `amaru` taken on crates.io and npm; PyPI `amaru` returned 404 (name free there). |
| Domains `amaru.dev/.io/.ai/.app/.global` | 2026-10-26 | All resolve to existing sites (page titles only; ownership not checked). `amaru.com` did not resolve. |

## Findings: other uses of "Amaru"
These are facts about public software and sites, not conclusions about trademark rights.
1. **`amaru` on npm**: "Skills, commands & agents manager for Claude Code (npm wrapper for the Go binary)", MIT, latest 0.6.1, repository `useamaru/amaru`, modified 2026-09-08. **Closest overlap**: developer tooling around AI coding agents, the field Amaru targets.
2. **Amaru, a Cardano blockchain node** (`pragma-org/amaru`, Rust, Apache-2.0, ~119 stars, pushed 2026-10-26, site `amaru.global`); `amaru` on crates.io is the same project. Active, software, different field.
3. `hibeam-dev/amaru_agent` ("Official agent reference implementation for Amaru", Go, Apache-2.0, last push 2026-05-08): relationship to the items above or to any product unverified.
4. `NumSoftware/Amaru.jl` (finite element library, Julia) and `CripterHack/Amaru` (antivirus for Windows 11, Rust): software, different fields.
5. `amaru.ai` presents "Amaru Robotics: AI packing robots for fulfillment", a company using the name with "AI" in an unrelated field.
6. The word is also a common given name and the Quechua word for a serpent/dragon (Túpac Amaru); this affects distinctiveness arguments but was not analyzed.

## Limitations
- No official trademark register was queried. Absence of search hits is not evidence that no mark exists.
- Only exact "Amaru" was searched; phonetic and spelling variants (Amaro, Amarú, Amaroo) and translations were not.
- No class analysis (software is typically Nice classes 9, 42), no jurisdiction analysis (the product owner is in Brazil; the target market is global), and no common-law (unregistered) use research beyond what is listed.
- Registry and GitHub counts are a snapshot and unverified for ownership, activity or trademark status.

## Residual uncertainty and required decision
- **Requires decision (product owner):** whether to (a) commission or personally run searches in the official registers (USPTO, EUIPO/TMview, WIPO, INPI) and have counsel read them, (b) keep the name given findings 1–2, or (c) start a rename through an ADR before any public release ([Q34](../OPEN_QUESTIONS.md#q34-product-name-and-trademark): "A conflict leads to a rename through an ADR").
- The npm package `amaru` also means the unscoped name is not available for an Amaru CLI package on npm; package naming is a separate decision for the WP that publishes one.
- Nothing here changes any ADR. No rename is proposed by this report.
