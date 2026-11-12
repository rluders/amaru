---
title: Keep tool-native Skill discovery and import in M7
status: accepted
date: 2026-11-12
deciders: Ricardo Lüders
supersedes: 0064
---

# 0065. Keep tool-native Skill discovery and import in M7

## Context

**Accepted by the product owner; discovery/import remains unimplemented.** PR #343 merged into #18 at `72f3e53576d6e08f042fd002aab55d97e6b1e5a6`. Project settings now have domain/store/application, guarded HTTP and generated SDK integration evidence. Product startup and full #191 acceptance remain incomplete.

The [feature catalog](../../product/FEATURE_CATALOG.md#27-skills-and-context-skl) schedules FEAT-SKL-003, opt-in import of tool-native Skills, for M7. [FEAT-PRJ-002](../../product/FEATURE_CATALOG.md#31-projects-and-work) requires Project-isolated settings, Executions and Events plus independent progress and secret isolation; it does not explicitly require an importer. However, [then-accepted ADR-0064](0064-project-discovery-settings-live-in-platform-state.md#consequences) requires demonstrating actual discovery at two attached roots before claiming full #191. The default-off preference does not itself implement that feature. This is a scheduling conflict, not evidence that discovery works.

Existing [SkillStore](../../../services/platform/internal/ports/skill.go) accepts immutable name/version/body records and rejects different content under an existing version. [NewSkill](../../../services/platform/internal/domain/skill.go) requires an authorized human and computes a body hash. Neither supplies a filesystem discovery/import mapping. The [.amaru design](../AMARU_DIRECTORY.md#2-layout) references Skill paths but remains proposed. A scanner would still need reviewed source/revision selection, confinement and symlink rules, size bounds, parsing, name/version collision behavior and content-change handling. No predecessor implementation was inspected or imported for this proposal.

## Decision

**Accepted by the product owner.**

1. Keep tool-native Skill discovery and import, including scanning `.claude/skills/*/SKILL.md`, in FEAT-SKL-003/M7. Do not add an M3 scanner, importer, file grammar, automatic SkillStore registration or discovery endpoint to satisfy the existing boolean.
2. Retain ADR-0064's implemented default-off Platform preference, attached-Project scope, shared-clone identity, authenticated reads, atomic unversioned patches and separation from trust and execution configuration. Enabling the preference records intent only until discovery is implemented; it currently discovers nothing. It must never grant trust or permission.
3. Replace only ADR-0064's requirement for actual discovery before full #191 acceptance. For M3, verify settings isolation through authenticated product composition and ensure rejected/foreign writes cannot mutate either Project or create trust. Defer two-root discovery tests to M7; when discovery is implemented, prove the preference is honored and content-bound trust still gates use. Existing manually registered, hash-pinned Skill behavior remains unchanged.
4. Preserve every other #191/#18 acceptance gate: product composition, Project-selected definitions, authoritative Work/dependency isolation, exact-scope secrets and authorized encrypted delivery, clone lookup, independent progress and composed two-client API/SDK checks. This decision alone closes neither issue nor M3.
5. This ADR supersedes ADR-0064, carrying forward its unchanged settings decision above and replacing only its M3 actual-discovery prerequisite. ADR-0064 remains the historical detail of those settings semantics; its substance is preserved. Update the decision index, Q63/Q64, current delivery documentation and #191/#18 tracking together.

## Consequences

M3 keeps the existing settings resource without pulling an M7 feature forward. Its enabled value has no discovery effect today; product documentation must say so. M7 must design safe discovery and explicit import into the immutable Skill model before implementation, using existing trust and Policy boundaries. This decision approves no external format, dependency, infrastructure or predecessor migration.

**Architecture review: accepted by the product owner.** Platform ownership, pure domain code and Client/Worker boundaries are unchanged. There are no new runtime calls or writes, so no new crash/retry behavior. Existing committed settings survive adapter recreation; full product restart acceptance remains separate. Linux remains required for M3 and other OSes best effort. This scheduling decision applies equally to all deployment models; multi-Organization authorization remains a separate gate.

Validate integrated settings through the eventual product composition, not only the current fake-backed SDK fixture. Retain the discovery limitation in user-facing status. Revisit the schedule if actual M3 usage needs tool-native discovery; then approve a bounded design and explicitly move FEAT-SKL-003 rather than inferring it from a setting.

## Alternatives considered

- **Pull discovery/import into M3:** satisfies ADR-0064 literally but expands the catalog schedule and requires a security-sensitive file-to-Skill mapping. Choose this only through an explicit owner-approved scope change.
- **Add a read-only discovery preview now:** still requires source, confinement and parsing policy and a separately defined result; it does not establish safe import into immutable Skill records. No current public preview contract requires it.
- **Remove the settings field:** breaks the existing shared contract and discards completed isolation work. Retain the resource with an honest limitation.
- **Silently ignore ADR-0064's discovery gate:** would claim completion against an accepted decision without approval. Replace the gate only through this explicitly approved decision.

## Approval

Proposed by Codex on 2026-11-12 for #191/#18 after reviewing merged PR #343. Approved by Ricardo Lüders (product owner) on 2026-11-12 in the repository task conversation: "PR #344 proposal is approved. Update it, merge, and move to next task." Recorded in [PR #344](https://github.com/rluders/amaru/pull/344). Approval changes the completion gate; it is not evidence of discovery/import or full product acceptance.
