# Documentation

| Area | Documents |
|---|---|
| Product | [PRD](product/PRD.md) · [Glossary](product/GLOSSARY.md) |
| Architecture | [Architecture](architecture/ARCHITECTURE.md) · [Roadmap](architecture/ROADMAP.md) · [Decisions](architecture/DECISIONS.md) and [ADRs](architecture/adr/) · [Open questions](architecture/OPEN_QUESTIONS.md) |
| Engineering | [Conventions](engineering/CONVENTIONS.md) · [Repository layout](engineering/repository-layout.md) · [Validation](engineering/validation.md) · [Agent instructions](engineering/agent-instructions.md) |
| Security | [Security model](security/SECURITY_MODEL.md) |
| Migration | [Migration plan](migration/MIGRATION_PLAN.md) · [Weave inventory](migration/weave-inventory.md) |

Every statement about behavior is labelled with the [status vocabulary](engineering/CONVENTIONS.md#status-vocabulary). In short: nothing in Amaru is implemented except `tools/doccheck`; **[Weave]** marks behavior that exists in the predecessor repository; everything else is proposed or open.

Ownership: these documents are maintained with the code and decisions they describe (`documentation-maintenance` Skill). `make docs-check` validates links, anchors, fences, the ADR index and Skill adapters.
