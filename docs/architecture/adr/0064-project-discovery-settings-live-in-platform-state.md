---
title: Store Project discovery settings in Platform state
status: proposed
date: 2026-11-12
deciders:
---

# 0064. Store Project discovery settings in Platform state

## Context

After [PR #337](https://github.com/rluders/amaru/pull/337), [PRJ-002 #191](https://github.com/rluders/amaru/issues/191) still needs isolated settings. The authorized Project application boundary and committed Workflow loader exist; settings storage and public handlers do not. This proposal resolves the single existing discovery switch before implementation, rather than adopting the wider draft configuration format.

Verified at #18 integration `4f918078dc484b5a431eb05ac85bfa551b4f2fd6`:

- [ProjectSettings](../../../packages/contracts/schemas/ProjectSettings.json) contains one required boolean, `importClaudeSkills`. [UpdateSettings](../../../packages/contracts/schemas/UpdateSettings.json) permits a nonempty partial update. The switch only widens read-only discovery of `.claude/skills/*/SKILL.md`; discovered content remains untrusted.
- [OpenAPI](../../../packages/contracts/openapi/openapi.yaml) describes a Project `GET`/`PATCH` settings resource and a `.Amaru/settings.json` file; the schema description spells it `.amaru/settings.json`. Neither is a serving or persistence implementation.
- [ADR-0039](0039-configuration-scopes-and-resolution-order.md) and [ADR-0043](0043-project-definitions-in-git-teams-and-operational-state-in-platform.md) put execution configuration in committed `.amaru/config.yaml`, with authorization/trust and operational state in the Platform. [AMARU_DIRECTORY §3.1](../AMARU_DIRECTORY.md#31-configyaml) still labels its config fields proposed and defines no settings file.
- [ADR-0049](0049-adapt-the-prototype-domain-and-ports-into-services-platform.md) leaves the predecessor's `domain/settings.go` disposition to a human. [ADR-0055](0055-write-policy-grants-and-project-trust-in-amaru-with-the-prototype-as-reference.md) did not resolve it. [ADR-0059](0059-adapt-the-prototype-http-handlers-to-the-platform-public-api.md) permits selected handler adaptation, but cannot supply that missing domain/storage decision.
- [ADR-0062](0062-project-bound-public-operations-and-secret-resolution.md) requires current authorization before settings access, selected-Project isolation and no authority granted by definition loading. The pure [configuration resolver](../../../services/platform/internal/domain/config.go) has no persistence or Project authorization responsibility.

The choice is whether this discovery preference is a Git definition or Platform state. It does not choose the full configuration-file grammar, Skill discovery implementation or a secret backend.

## Decision

**Proposed; requires product-owner approval before implementation.**

1. **Keep discovery preferences in the Platform.** Store `importClaudeSkills` against the attached Project identity in the existing PostgreSQL adapter. Missing settings for an attached Project mean `false`; absent/unattached Projects return the same safe not-found result as the existing Project boundary. All clones of that Project on this Instance share the preference. Another Project, another Instance or a new attachment with a different Project identity does not inherit it. It is neither a user-wide default nor a credential/consent record.
2. **Preserve the existing resource shape.** Keep the boolean in `ProjectSettings` and partial-update semantics in `UpdateSettings`. Use the current authenticated administrator/Project boundary before reads and mutations; never accept an Organization, root path or identity from the settings payload. Reject empty, unknown-field and non-boolean updates without writes. Apply a patch atomically to the current stored settings, preserving unspecified fields. Concurrent patches serialize at the authoritative store; for this one switch, the last committed explicit value wins. A repeated patch sets the same value. Do not claim expected-version conflicts or original-outcome command replay for this unversioned resource; adding either requires a separate reviewed shared-contract change.
3. **Separate discovery from authority.** Enabling the switch permits the selected Project's eligible read-only Skill discovery only. It grants no trust, Capability, filesystem write, secret access or execution permission. Existing content-bound trust, Policy and Approval still apply before execution. Disabling it affects later discovery, without rewriting existing Execution snapshots. Repository content cannot enable the preference, and no fallback to another Project's setting is allowed.
4. **Keep execution configuration in Git.** `.amaru/config.yaml` remains the ADR-0039 Project layer. This discovery preference does not enter `ResolveConfig`, change lock/tighten precedence, override Organization Policy, or establish a new config grammar. Implement only the current switch; do not add proposed Agent, Local Check, webhook or security keys. Explicit definition edits remain separate actions; settings PATCH never writes a Project file.
5. **Write this small model fresh.** Resolve ADR-0049's `settings.go` disposition as **reimplement in Amaru, no predecessor code import** for the existing discovery switch. The committed Amaru schemas are its behavior reference. Other deferred domain files remain undecided. No existing settings-file importer, auto-detection or compatibility promise is introduced. The implementation changes misleading OpenAPI/schema storage descriptions together with generated SDK output and its staleness checks; wire field names and accepted payloads remain unchanged.

## Consequences

Project preference persistence becomes an explicit Platform responsibility. A small pure domain value, settings port, fake and PostgreSQL adapter can reuse existing layering and migration tooling; no new service, runtime dependency or infrastructure is needed. The Platform owns the value; Clients access only the public API and Workers hold no authoritative settings. Self-hosted/Cloud adapters must retain Organization isolation before multi-Organization exposure; compact M3 remains one configured Organization.

Settings intentionally do not travel with a Git clone or restore from repository content. Platform backup/restore carries them with operational state. A failed write or transaction rollback leaves the previous value unchanged; no successful response precedes commit. Restart reloads the committed value. No durable settings-change Event is promised by the present contract; a future notification must use an agreed typed envelope and commit atomically with the value. Error responses omit repository contents, credentials and foreign Project details.

Implementation and evidence required after approval:

- Add Project-scoped read/update application operations and a minimal store contract; derive authority before resource access, including repeated requests and revoked sessions. Use the existing PostgreSQL migration process, without altering the immutable Project identity.
- Add shared fake/PostgreSQL conformance cases: two attached Projects with the same switch name, default false, independent updates, missing/foreign Project refusal, rollback, concurrent updates and restart. Add authenticated application cases asserting rejected calls cannot mutate either Project.
- Update schema/OpenAPI descriptions, generated SDK and focused fixtures/tests; serve the existing GET/PATCH resource only through #18's guarded product composition. Before claiming full #191, demonstrate discovery honors the preference for two actual attached roots and that enabling discovery cannot grant content trust.
- Run focused race checks, isolated PostgreSQL tests when store/migration code changes, and `make check` on candidate and merged Linux results. macOS/Windows remain best effort for M3. A proposal check is documentation evidence, not settings behavior validation.

**Architecture review: needs human decision.** The proposal keeps the accepted Platform/Client/Worker and pure-domain boundaries, avoids implicit repository mutation, and makes concurrent updates and failure behavior explicit. Its trade-off is Instance-local preference persistence instead of Git portability. This ADR does not complete #191, #18 or M3; execution configuration, other definitions, Work dependencies, secrets, clone lookup and composed API acceptance remain independent gates. Revisit if an actual second setting needs versioned edits, Organization defaults or a Git-reviewable execution effect.

## Alternatives considered

- **Persist `.amaru/settings.json` as described by the inherited contract:** adds a second configuration file and repository writes to an operational PATCH without a reviewed revision/conflict model. It also lets copied repository content opt into broader discovery. Prefer explicit Platform preference storage.
- **Put this switch into `.amaru/config.yaml`:** Git portability is useful for execution definitions, but this needs approval of a file grammar and explicit Git-edit/reconciliation semantics before serving PATCH. Keep that wider configuration work separate from the one existing discovery preference.
- **Keep settings in memory or per user:** smaller initial implementation, but loses restart persistence or couples unrelated Projects; it cannot establish #191 settings isolation.

## Approval

Proposed by: Codex for #191 on 2026-11-12. Approved by: pending product-owner decision. No settings runtime implementation is authorized by this proposed ADR alone.
