# services/platform

Go module `github.com/rluders/amaru/services/platform`: the Platform core. Introduced by WP-M3-01 ([#13](https://github.com/rluders/amaru/issues/13)) under [ADR-0049](../../docs/architecture/adr/0049-adapt-the-prototype-domain-and-ports-into-services-platform.md), which moves selected code from the earlier internal implementation at baseline commit `c8a2f6e` ([ADR-0032](../../docs/architecture/adr/0032-migration-baseline-is-a-pinned-commit.md)). Provenance per file: [PROVENANCE.md](PROVENANCE.md).

## What exists (unit-tested, Linux only so far)
| Path | Content |
|---|---|
| `internal/domain` | Execution and Step Execution state machines (FEAT-EXE-001, [#118](https://github.com/rluders/amaru/issues/118)), per [DOMAIN_MODEL §5.1–5.2](../../docs/architecture/DOMAIN_MODEL.md#5-lifecycles), with the restart rule of [ADR-0042](../../docs/architecture/adr/0042-durable-execution-with-explicit-restart-semantics.md)/[ADR-0044](../../docs/architecture/adr/0044-restart-semantics-clarifications-from-the-m1-spike.md). Standard library only. |
| `internal/arch` | Architecture test: the core imports no OS, process, network, vendor or adapter package (negative-tested by adding a forbidden import). |

Nothing else of the Platform exists yet: no storage, API, coordinator or Worker. Approval states, Work Items, Agents and Capabilities arrive with the other sub-issues of #13.

## Check
`make platform-test` (part of `make check`). Tests use no real agents, network or paid APIs.
