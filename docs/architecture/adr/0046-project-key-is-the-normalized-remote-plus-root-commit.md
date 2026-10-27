---
title: A Project is keyed by normalized remote URL plus root commit; Linked repositories are committed definitions; Team slugs are immutable
status: accepted
date: 2026-10-26
deciders: Ricardo Lüders
---

# 0046. A Project is keyed by normalized remote URL plus root commit; Linked repositories are committed definitions; Team slugs are immutable

## Context
- [ADR-0043](0043-project-definitions-in-git-teams-and-operational-state-in-platform.md) keys a Project by repository, records the clone, commit SHA, content hash and definition snapshot per Execution, and validates Team slugs. It does not say how the repository key is derived, whether Linked repositories are committed, or what happens to a Team slug that committed definitions still reference.
- The options and trade-offs are in [AMARU_DIRECTORY §5.1](../AMARU_DIRECTORY.md#51-project-key); the question is [Q55](../OPEN_QUESTIONS.md#q55-project-key-and-linked-repository-placement).
- Verified: nothing here is implemented. The decision is checked at M3 with two clones against one Platform ([ADR-0043](0043-project-definitions-in-git-teams-and-operational-state-in-platform.md)).

## Decision
- **Project key.** The key is the normalized remote URL (host lowercased; credentials, `.git` suffix and trailing slash removed; transport normalized to `ssh`, see the amendment below) plus the repository's root commit SHA. Any clone derives the same key.
- **No remote.** A repository without a remote carries a committed Project id (a ULID) in `.amaru/config.yaml`, generated on `attach` and reviewed through Git, and used as the key.
- **Aliases.** A second remote that normalizes differently is linked to the Project by an explicit user action. Amaru never merges Projects automatically. A fork is a different Project unless the user links it.
- **Linked repositories** are committed definitions in `.amaru/repositories.yaml`: `id`, `remote`, `baseBranch`, `deliveryPolicy`. Absolute local paths stay in the Platform's clone record.
- **Team slugs are immutable.** A Team is renamed by creating a new Team and deprecating the old one. A Team referenced by a definition at a known revision of the Project's default branch cannot be deleted without an explicit force that lists the referencing Projects. New Executions fail validation on a deleted slug; running Executions keep the Team ids resolved at start.

## Consequences
- Clone location, path spelling and credentials do not change a Project's identity; two clones share operational state.
- A transferred or renamed remote yields a new key until the user links the alias; this is manual by design.
- The importer ([LEGACY_IMPORTER](../LEGACY_IMPORTER.md)) maps prototype repository entries to `.amaru/repositories.yaml`.
- Revisit if root-commit disambiguation proves unreliable (for example rewritten history) at the M3 two-clone validation.

## Alternatives considered
- **Remote URL only:** a rename, transfer or mirror breaks or collides identity.
- **Committed Project id for every repository:** survives remote changes, but a copied repository duplicates the id and every Project must commit one.
- **Linked repositories only in the Platform:** hides delivery policy from review; only absolute paths are truly per clone.

## Approval
Proposed by: Claude (WP-M1-05, 2026-10-26) · Approved by: Ricardo Lüders on 2026-10-26, in the session that delivered WP-M1-05 (explicit acceptance after the proposal in PR #240).

## Amendment 2026-10-27: ssh is the canonical transport
The original text lowercased the scheme but left open whether `https://host/acme/app` and `git@host:acme/app` are the same Project. The product owner decided: "always ssh over https". The normalized remote therefore has the form `ssh://<host>/<path>`: `https`, `http`, `git` and scp-like (`user@host:path`) remotes map onto it, and the port of a web or git-protocol URL is dropped because it says nothing about the ssh port. An explicit ssh port other than 22 is kept. One repository cloned over https and over ssh now has one key. Implemented in `services/platform` (`domain.NormalizeRemote`, unit-tested); nothing else changes in the Decision. Approved by: Ricardo Lüders, 2026-10-27, in the WP-M3-02 session.
