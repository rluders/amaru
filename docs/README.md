# Documentation

| Area | Documents |
|---|---|
| Product | [PRD](product/PRD.md) · [Feature catalog](product/FEATURE_CATALOG.md) · [Non-functional requirements](product/NON_FUNCTIONAL_REQUIREMENTS.md) · [Glossary](product/GLOSSARY.md) |
| Architecture | [Architecture (hub)](architecture/ARCHITECTURE.md) · [Domain model](architecture/DOMAIN_MODEL.md) · [Extensibility](architecture/EXTENSIBILITY.md) · [Execution environments](architecture/EXECUTION_ENVIRONMENTS.md) · [Data architecture](architecture/DATA_ARCHITECTURE.md) · [Deployment](architecture/DEPLOYMENT.md) · [Clients](architecture/CLIENTS.md) · [Resilience](architecture/RESILIENCE.md) · [Technology benchmark](architecture/TECHNOLOGY_BENCHMARK.md) |
| Decisions | [Decisions and ADR backlog](architecture/DECISIONS.md) and [ADRs](architecture/adr/) · [Open questions](architecture/OPEN_QUESTIONS.md) |
| Delivery | [Master roadmap](architecture/ROADMAP.md) · [Work Packages](architecture/WORK_PACKAGES.md) |
| Engineering | [Conventions](engineering/CONVENTIONS.md) · [Repository layout](engineering/repository-layout.md) · [Validation](engineering/validation.md) · [Agent instructions](engineering/agent-instructions.md) |
| Security | [Security model](security/SECURITY_MODEL.md) |
| Migration | [Migration plan](migration/MIGRATION_PLAN.md) · [Repository assessment](migration/REPOSITORY_ASSESSMENT.md) · [Weave inventory](migration/weave-inventory.md) · [Weave prototype inventory](migration/weave-prototype-inventory.md) |

Suggested reading order:
1. [PRD](product/PRD.md)
2. [Architecture](architecture/ARCHITECTURE.md)
3. [Master roadmap](architecture/ROADMAP.md)
4. [Repository assessment](migration/REPOSITORY_ASSESSMENT.md)

Every statement about behavior is labelled with the [status vocabulary](engineering/CONVENTIONS.md#status-vocabulary):
- Nothing in Amaru is implemented except `tools/doccheck`.
- **[Weave]** marks behavior that exists in the predecessor Runtime (`weave-obsidian`).
- **[Weave prototype]** marks behavior in the earlier `weave` repository.
- Everything else is **proposed**: decided (accepted ADRs, accepted planning documents) but not built. No question is open ([OPEN_QUESTIONS](architecture/OPEN_QUESTIONS.md)).

**Ownership.** These documents are maintained with the code and decisions they describe (`documentation-maintenance` Skill). `make docs-check` validates links, anchors, fences, the ADR index and Skill adapters. Feature IDs and milestone traceability are not checked by tooling yet ([roadmap § Feature traceability](architecture/ROADMAP.md#feature-traceability)).
