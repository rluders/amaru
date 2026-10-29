# Documentation

| Area | Documents |
|---|---|
| Product | [PRD](product/PRD.md) · [Feature catalog](product/FEATURE_CATALOG.md) · [Non-functional requirements](product/NON_FUNCTIONAL_REQUIREMENTS.md) · [Glossary](product/GLOSSARY.md) |
| Architecture | [Architecture (hub)](architecture/ARCHITECTURE.md) · [Domain model](architecture/DOMAIN_MODEL.md) · [Extensibility](architecture/EXTENSIBILITY.md) · [Execution environments](architecture/EXECUTION_ENVIRONMENTS.md) · [Data architecture](architecture/DATA_ARCHITECTURE.md) · [`.amaru/` layout](architecture/AMARU_DIRECTORY.md) · [Deployment](architecture/DEPLOYMENT.md) · [Policy and Approval shapes](architecture/POLICY_AND_APPROVAL_SHAPES.md) · [Clients](architecture/CLIENTS.md) · [Contract versioning](architecture/CONTRACT_VERSIONING.md) · [Worker protocol](architecture/WORKER_PROTOCOL.md) · [Resilience](architecture/RESILIENCE.md) · [Technology benchmark](architecture/TECHNOLOGY_BENCHMARK.md) |
| Decisions | [Decisions and ADR backlog](architecture/DECISIONS.md) and [ADRs](architecture/adr/) · [Open questions](architecture/OPEN_QUESTIONS.md) |
| Delivery | [GitHub Project](https://github.com/users/rluders/projects/2) · [Master roadmap](architecture/ROADMAP.md) · [Work Packages](architecture/WORK_PACKAGES.md) · [Delivery workflow](engineering/delivery.md#9-project-and-operating-model) · [Backlog map](engineering/backlog-map.md) |
| Engineering | [Conventions](engineering/CONVENTIONS.md) · [Repository layout](engineering/repository-layout.md) · [Validation](engineering/validation.md) · [Agent instructions](engineering/agent-instructions.md) |
| Security | [Security model](security/SECURITY_MODEL.md) |

Suggested reading order:
1. [PRD](product/PRD.md)
2. [Architecture](architecture/ARCHITECTURE.md)
3. [Master roadmap](architecture/ROADMAP.md)

Every statement about behavior is labelled with the [status vocabulary](engineering/CONVENTIONS.md#status-vocabulary):
- M2 contracts/SDK and parts of the M3 Platform core are implemented with the evidence recorded in their issues and validation records; M3 remains incomplete. See the [Project](https://github.com/users/rluders/projects/2) and [roadmap](architecture/ROADMAP.md).
- **[Prototype]** marks behavior that exists in an earlier internal prototype, not in Amaru.
- Accepted architecture is not implementation evidence. Future capabilities remain **proposed**; consult [open questions and recorded decisions](architecture/OPEN_QUESTIONS.md) rather than assuming every question is resolved.

**Ownership.** These documents are maintained with the code and decisions they describe (`documentation-maintenance` Skill). `make docs-check` validates links, anchors, fences, the ADR index and Skill adapters. Feature IDs and milestone traceability are not checked by tooling yet ([roadmap § Feature traceability](architecture/ROADMAP.md#feature-traceability)).
