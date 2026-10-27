# Backlog Map

Planning IDs mapped to their GitHub issues in [rluders/amaru](https://github.com/rluders/amaru/issues). Generated when the backlog was initialized (2026-10-25); keep it in step with the issues as described in the [delivery workflow](delivery.md#7-traceability-and-change). Scope and acceptance live in the planning documents and the issues; this file holds only the mapping.

## 1. Work Packages, epics and other tracked items

| Milestone | Planning ID | Issue | Parent | Title |
|---|---|---|---|---|
| M0 | `WP-M0-01` | [#1](https://github.com/rluders/amaru/issues/1) | — | WP-M0-01 Planning package review |
| M1 | `WP-M1-01` | [#2](https://github.com/rluders/amaru/issues/2) | — | WP-M1-01 License texts in effect |
| M1 | `WP-M1-02` | [#3](https://github.com/rluders/amaru/issues/3) | — | WP-M1-02 Storage and execution validation spike |
| M1 | `WP-M1-03` | [#4](https://github.com/rluders/amaru/issues/4) | — | WP-M1-03 Contract versioning rules in fixtures |
| M1 | `WP-M1-04` | [#5](https://github.com/rluders/amaru/issues/5) | — | WP-M1-04 Approval, Policy, roles and configuration shapes |
| M1 | `WP-M1-05` | [#6](https://github.com/rluders/amaru/issues/6) | — | WP-M1-05 `.amaru/` layout design |
| M1 | `WP-M1-06` | [#7](https://github.com/rluders/amaru/issues/7) | — | WP-M1-06 Worker protocol schemas |
| M1 | `WP-M1-07` | [#8](https://github.com/rluders/amaru/issues/8) | — | WP-M1-07 Inventory refresh at the baseline |
| M1 | `OWN-legal-review` | [#65](https://github.com/rluders/amaru/issues/65) | [#2](https://github.com/rluders/amaru/issues/2) | Legal review of the Amaru Community License 1.0 draft |
| M1 | `OWN-license-files` | [#66](https://github.com/rluders/amaru/issues/66) | — | Add LICENSE files (community license at root, Apache-2.0 for contracts and SDKs) |
| M1 | `OWN-cla` | [#69](https://github.com/rluders/amaru/issues/69) | [#2](https://github.com/rluders/amaru/issues/2) | CLA text and signing tooling |
| M1 | `OWN-trademark` | [#70](https://github.com/rluders/amaru/issues/70) | [#2](https://github.com/rluders/amaru/issues/2) | Trademark search for "Amaru" |
| M1 | `OWN-security-md` | [#71](https://github.com/rluders/amaru/issues/71) | [#2](https://github.com/rluders/amaru/issues/2) | Vulnerability disclosure policy (SECURITY.md) |
| M1 | `SPK-postgres` | [#72](https://github.com/rluders/amaru/issues/72) | [#3](https://github.com/rluders/amaru/issues/3) | Spike: bundled, supervised PostgreSQL on Windows, macOS and Linux (A2, A12) |
| M1 | `SPK-nats-outbox` | [#73](https://github.com/rluders/amaru/issues/73) | [#3](https://github.com/rluders/amaru/issues/3) | Spike: embedded NATS JetStream with outbox relay (A9) |
| M1 | `SPK-crash-suite` | [#74](https://github.com/rluders/amaru/issues/74) | [#3](https://github.com/rluders/amaru/issues/3) | Spike: representative Workflow under crash injection, Approval wait, timeouts and leases |
| M1 | `EXIT-M1` | [#60](https://github.com/rluders/amaru/issues/60) | — | M1 Decisions and spikes: exit validation |
| M2 | `WP-M2-01` | [#9](https://github.com/rluders/amaru/issues/9) | — | WP-M2-01 Contracts package from the prototype protocol |
| M2 | `WP-M2-02` | [#10](https://github.com/rluders/amaru/issues/10) | — | WP-M2-02 Generated TypeScript SDK |
| M2 | `WP-M2-03` | [#11](https://github.com/rluders/amaru/issues/11) | — | WP-M2-03 Contract drift guard |
| M2 | `MADR-protocol` | [#67](https://github.com/rluders/amaru/issues/67) | [#9](https://github.com/rluders/amaru/issues/9) | Migration ADR: protocol/ → packages/contracts |
| M2 | `EXIT-M2` | [#12](https://github.com/rluders/amaru/issues/12) | — | M2 Contracts v0: exit validation |
| M3 | `WP-M3-01` | [#13](https://github.com/rluders/amaru/issues/13) | — | WP-M3-01 Domain and ports migration |
| M3 | `WP-M3-02` | [#14](https://github.com/rluders/amaru/issues/14) | — | WP-M3-02 Storage adapter |
| M3 | `WP-M3-03` | [#15](https://github.com/rluders/amaru/issues/15) | — | WP-M3-03 Execution coordinator with embedded Worker |
| M3 | `WP-M3-04` | [#16](https://github.com/rluders/amaru/issues/16) | — | WP-M3-04 Policy, Grants and Approval |
| M3 | `WP-M3-05` | [#17](https://github.com/rluders/amaru/issues/17) | — | WP-M3-05 Providers and context assembly |
| M3 | `WP-M3-06` | [#18](https://github.com/rluders/amaru/issues/18) | — | WP-M3-06 Public API, events and local authentication |
| M3 | `WP-M3-07` | [#19](https://github.com/rluders/amaru/issues/19) | — | WP-M3-07 Worktrees, configuration scopes, evidence |
| M3 | `MADR-domain` | [#75](https://github.com/rluders/amaru/issues/75) | [#13](https://github.com/rluders/amaru/issues/13) | Migration ADR: runtime/internal/domain → services/platform` core |
| M3 | `MADR-store` | [#76](https://github.com/rluders/amaru/issues/76) | [#14](https://github.com/rluders/amaru/issues/14) | Migration ADR: runtime/internal/store/* → storage adapters (PostgreSQL) |
| M3 | `MADR-app` | [#77](https://github.com/rluders/amaru/issues/77) | [#15](https://github.com/rluders/amaru/issues/15) | Migration ADR: runtime/internal/app → Execution coordinator |
| M3 | `MADR-platform` | [#78](https://github.com/rluders/amaru/issues/78) | [#15](https://github.com/rluders/amaru/issues/15) | Migration ADR: runtime/internal/platform → Worker OS layer |
| M3 | `MADR-provider-remotejob` | [#79](https://github.com/rluders/amaru/issues/79) | [#15](https://github.com/rluders/amaru/issues/15) | Migration ADR: runtime/internal/provider/remotejob → Worker providers |
| M3 | `MADR-provider-claude` | [#80](https://github.com/rluders/amaru/issues/80) | [#17](https://github.com/rluders/amaru/issues/17) | Migration ADR: runtime/internal/provider/claude → Worker providers |
| M3 | `MADR-provider-codex` | [#81](https://github.com/rluders/amaru/issues/81) | [#17](https://github.com/rluders/amaru/issues/17) | Migration ADR: runtime/internal/provider/codex → Worker providers |
| M3 | `MADR-provider-opencode` | [#82](https://github.com/rluders/amaru/issues/82) | [#17](https://github.com/rluders/amaru/issues/17) | Migration ADR: runtime/internal/provider/opencode → Worker providers |
| M3 | `MADR-httpapi` | [#83](https://github.com/rluders/amaru/issues/83) | [#18](https://github.com/rluders/amaru/issues/18) | Migration ADR: runtime/internal/transport/httpapi → Platform API |
| M3 | `MADR-auth-discovery` | [#84](https://github.com/rluders/amaru/issues/84) | [#18](https://github.com/rluders/amaru/issues/18) | Migration ADR: runtime/internal/{auth,discovery} → compact deployment adapter |
| M3 | `MADR-provider-gitrepo` | [#85](https://github.com/rluders/amaru/issues/85) | [#19](https://github.com/rluders/amaru/issues/19) | Migration ADR: runtime/internal/provider/gitrepo → Worker providers (worktrees) |
| M3 | `EXIT-M3` | [#20](https://github.com/rluders/amaru/issues/20) | — | M3 Platform core: exit validation |
| M4 | `WP-M4-01` | [#21](https://github.com/rluders/amaru/issues/21) | — | WP-M4-01 Web shell and design system |
| M4 | `WP-M4-02` | [#22](https://github.com/rluders/amaru/issues/22) | — | WP-M4-02 Core screens |
| M4 | `WP-M4-03` | [#23](https://github.com/rluders/amaru/issues/23) | — | WP-M4-03 Delivery loop migration |
| M4 | `WP-M4-04` | [#24](https://github.com/rluders/amaru/issues/24) | — | WP-M4-04 Checks, CI and review |
| M4 | `WP-M4-05` | [#25](https://github.com/rluders/amaru/issues/25) | — | WP-M4-05 Forge delivery |
| M4 | `WP-M4-06` | [#26](https://github.com/rluders/amaru/issues/26) | — | WP-M4-06 Approval inbox, diff and cost |
| M4 | `WP-M4-07` | [#27](https://github.com/rluders/amaru/issues/27) | — | WP-M4-07 Packaging and MVP validation |
| M4 | `MADR-dashboard` | [#86](https://github.com/rluders/amaru/issues/86) | [#21](https://github.com/rluders/amaru/issues/21) | Migration ADR: dashboard/ → apps/web` → `packages/ui |
| M4 | `MADR-autoloop` | [#87](https://github.com/rluders/amaru/issues/87) | [#23](https://github.com/rluders/amaru/issues/23) | Migration ADR: runtime/internal/app/autoloop* → Delivery loop |
| M4 | `MADR-provider-github` | [#88](https://github.com/rluders/amaru/issues/88) | [#25](https://github.com/rluders/amaru/issues/25) | Migration ADR: runtime/internal/provider/github → forge port |
| M4 | `SEC-ADR0034` | [#105](https://github.com/rluders/amaru/issues/105) | [#27](https://github.com/rluders/amaru/issues/27) | Security review: local Web authentication (ADR-0034) |
| M5 | `E5.5` | [#28](https://github.com/rluders/amaru/issues/28) | — | E5.5 UE5 + MCP pilot on a Windows VM |
| M5 | `E5.4` | [#29](https://github.com/rluders/amaru/issues/29) | — | E5.4 Sandbox Environment (single-level rootless containers) |
| M5 | `E5.1` | [#31](https://github.com/rluders/amaru/issues/31) | — | E5.1 Worker protocol implementation |
| M5 | `E5.2` | [#32](https://github.com/rluders/amaru/issues/32) | — | E5.2 Environment registry and routing |
| M5 | `E5.3` | [#33](https://github.com/rluders/amaru/issues/33) | — | E5.3 Leases and partition tests |
| M5 | `E5.6` | [#34](https://github.com/rluders/amaru/issues/34) | — | E5.6 Retries, parallelism and scheduling |
| M5 | `E5.7` | [#222](https://github.com/rluders/amaru/issues/222) | — | E5.7 Delivery efficiency (context budget, bounded CI repair) |
| M5 | `MADR-provider-mcp` | [#89](https://github.com/rluders/amaru/issues/89) | [#28](https://github.com/rluders/amaru/issues/28) | Migration ADR: runtime/internal/provider/mcp → Worker providers (MCP gateway) |
| M5 | `MADR-sandbox-runtime` | [#90](https://github.com/rluders/amaru/issues/90) | [#29](https://github.com/rluders/amaru/issues/29) | Migration ADR: runtime/{driver,cli_driver,podman}.go → Sandbox Environment Worker |
| M5 | `SPK-A5-A8` | [#103](https://github.com/rluders/amaru/issues/103) | [#28](https://github.com/rluders/amaru/issues/28) | Spike: interactive-session Worker and Unreal MCP coverage (A5, A8) |
| M5 | `EXIT-M5` | [#35](https://github.com/rluders/amaru/issues/35) | — | M5 Remote execution and execution hardening: exit validation |
| M6 | `E6.6` | [#30](https://github.com/rluders/amaru/issues/30) | — | E6.6 Adapter components |
| M6 | `E6.1` | [#36](https://github.com/rluders/amaru/issues/36) | — | E6.1 Plugin manifest schema and registry |
| M6 | `E6.2` | [#37](https://github.com/rluders/amaru/issues/37) | — | E6.2 Plugin permissions and consent |
| M6 | `E6.3` | [#38](https://github.com/rluders/amaru/issues/38) | — | E6.3 TypeScript plugin SDK |
| M6 | `E6.4` | [#39](https://github.com/rluders/amaru/issues/39) | — | E6.4 UI extension host |
| M6 | `E6.5` | [#40](https://github.com/rluders/amaru/issues/40) | — | E6.5 Logic extension host |
| M6 | `MADR-pandoc-ffmpeg` | [#91](https://github.com/rluders/amaru/issues/91) | [#30](https://github.com/rluders/amaru/issues/30) | Migration ADR: provider/{pandoc,ffmpeg} → example `worker-adapter` plugins |
| M6 | `SPK-A10` | [#104](https://github.com/rluders/amaru/issues/104) | [#40](https://github.com/rluders/amaru/issues/40) | Spike: Javy + Amaru Javy plugin toolchain and plugin-host process (A10) |
| M6 | `EXIT-M6` | [#41](https://github.com/rluders/amaru/issues/41) | — | M6 Plugin platform: exit validation |
| M7 | `E7.4` | [#42](https://github.com/rluders/amaru/issues/42) | — | E7.4 Team workflows (templates, separation of duties, intake) |
| M7 | `E7.1` | [#43](https://github.com/rluders/amaru/issues/43) | — | E7.1 Identity and Organizations |
| M7 | `E7.2` | [#44](https://github.com/rluders/amaru/issues/44) | — | E7.2 External PostgreSQL, NATS and S3-compatible configuration |
| M7 | `E7.3` | [#45](https://github.com/rluders/amaru/issues/45) | — | E7.3 Packaging, upgrade and restore |
| M7 | `E7.5` | [#46](https://github.com/rluders/amaru/issues/46) | — | E7.5 Observability and cost attribution |
| M7 | `E7.6` | [#47](https://github.com/rluders/amaru/issues/47) | — | E7.6 Command-line client |
| M7 | `E7.7` | [#48](https://github.com/rluders/amaru/issues/48) | — | E7.7 Security disclosure process |
| M7 | `E7.8` | [#49](https://github.com/rluders/amaru/issues/49) | — | E7.8 Environment templates and provisioning |
| M7 | `MADR-webhook` | [#95](https://github.com/rluders/amaru/issues/95) | [#42](https://github.com/rluders/amaru/issues/42) | Migration ADR: runtime/internal/transport/webhook → Platform API (webhooks) |
| M7 | `EXIT-M7` | [#55](https://github.com/rluders/amaru/issues/55) | — | M7 Self-hosted: exit validation |
| M8 | `E8.1` | [#50](https://github.com/rluders/amaru/issues/50) | — | E8.1 Tauri validation spike (ADR-0018) |
| M8 | `E8.2` | [#51](https://github.com/rluders/amaru/issues/51) | — | E8.2 Shared UI extraction and Desktop host |
| M8 | `E8.3` | [#52](https://github.com/rluders/amaru/issues/52) | — | E8.3 Instance and credential management |
| M8 | `E8.4` | [#53](https://github.com/rluders/amaru/issues/53) | — | E8.4 Local Platform supervision and updates |
| M8 | `E8.5` | [#54](https://github.com/rluders/amaru/issues/54) | — | E8.5 Desktop signing and release |
| M8 | `EXIT-M8` | [#61](https://github.com/rluders/amaru/issues/61) | — | M8 Desktop: exit validation |
| M9 | `E9.1` | [#56](https://github.com/rluders/amaru/issues/56) | — | E9.1 Threat model and cross-tenant isolation test suite |
| M9 | `E9.2` | [#57](https://github.com/rluders/amaru/issues/57) | — | E9.2 Cloud adapters |
| M9 | `E9.4` | [#58](https://github.com/rluders/amaru/issues/58) | — | E9.4 Cloud operations and SLOs |
| M9 | `E9.3` | [#59](https://github.com/rluders/amaru/issues/59) | — | E9.3 Managed Worker pools |
| M9 | `EXIT-M9` | [#62](https://github.com/rluders/amaru/issues/62) | — | M9 Cloud: exit validation |
| M10 | `EXIT-M10` | [#64](https://github.com/rluders/amaru/issues/64) | — | M10 Ecosystem: exit validation |

## 2. Features

Every scheduled feature (P0, P1, and P2 with a milestone). "Covered by" means the feature is folded into a Work Package or epic issue that covers only that feature.

| Feature | Milestone | Issue | Parent |
|---|---|---|---|
| FEAT-API-001 | M2 | [#68](https://github.com/rluders/amaru/issues/68) | [#9](https://github.com/rluders/amaru/issues/9) |
| FEAT-API-002 | M2 | [#117](https://github.com/rluders/amaru/issues/117) | [#9](https://github.com/rluders/amaru/issues/9) |
| FEAT-API-003 | M2 | covered by [#10](https://github.com/rluders/amaru/issues/10) (`WP-M2-02`) | — |
| FEAT-AGT-001 | M3 | [#110](https://github.com/rluders/amaru/issues/110) | [#13](https://github.com/rluders/amaru/issues/13) |
| FEAT-AGT-002 | M3 | [#112](https://github.com/rluders/amaru/issues/112) | [#17](https://github.com/rluders/amaru/issues/17) |
| FEAT-AGT-003 | M3 | [#113](https://github.com/rluders/amaru/issues/113) | [#17](https://github.com/rluders/amaru/issues/17) |
| FEAT-AGT-004 | M3 | [#114](https://github.com/rluders/amaru/issues/114) | [#13](https://github.com/rluders/amaru/issues/13) |
| FEAT-API-005 | M3 | [#106](https://github.com/rluders/amaru/issues/106) | [#18](https://github.com/rluders/amaru/issues/18) |
| FEAT-ART-001 | M3 | [#119](https://github.com/rluders/amaru/issues/119) | [#19](https://github.com/rluders/amaru/issues/19) |
| FEAT-ART-002 | M3 | [#124](https://github.com/rluders/amaru/issues/124) | [#19](https://github.com/rluders/amaru/issues/19) |
| FEAT-ART-003 | M3 | [#125](https://github.com/rluders/amaru/issues/125) | [#19](https://github.com/rluders/amaru/issues/19) |
| FEAT-CAP-001 | M3 | [#111](https://github.com/rluders/amaru/issues/111) | [#13](https://github.com/rluders/amaru/issues/13) |
| FEAT-CAP-002 | M3 | [#127](https://github.com/rluders/amaru/issues/127) | [#17](https://github.com/rluders/amaru/issues/17) |
| FEAT-CAP-003 | M3 | [#128](https://github.com/rluders/amaru/issues/128) | [#17](https://github.com/rluders/amaru/issues/17) |
| FEAT-CAP-004 | M3 | [#130](https://github.com/rluders/amaru/issues/130) | [#15](https://github.com/rluders/amaru/issues/15) |
| FEAT-CAP-005 | M3 | [#131](https://github.com/rluders/amaru/issues/131) | [#17](https://github.com/rluders/amaru/issues/17) |
| FEAT-ENV-001 | M3 | [#145](https://github.com/rluders/amaru/issues/145) | [#15](https://github.com/rluders/amaru/issues/15) |
| FEAT-ENV-010 | M3 | [#154](https://github.com/rluders/amaru/issues/154) | [#19](https://github.com/rluders/amaru/issues/19) |
| FEAT-EXE-001 | M3 | [#118](https://github.com/rluders/amaru/issues/118) | [#13](https://github.com/rluders/amaru/issues/13) |
| FEAT-EXE-002 | M3 | [#129](https://github.com/rluders/amaru/issues/129) | [#15](https://github.com/rluders/amaru/issues/15) |
| FEAT-EXE-003 | M3 | [#152](https://github.com/rluders/amaru/issues/152) | [#15](https://github.com/rluders/amaru/issues/15) |
| FEAT-EXE-004 | M3 | [#135](https://github.com/rluders/amaru/issues/135) | [#18](https://github.com/rluders/amaru/issues/18) |
| FEAT-EXE-005 | M3 | [#157](https://github.com/rluders/amaru/issues/157) | [#15](https://github.com/rluders/amaru/issues/15) |
| FEAT-EXE-006 | M3 | [#158](https://github.com/rluders/amaru/issues/158) | [#15](https://github.com/rluders/amaru/issues/15) |
| FEAT-IAM-001 | M3 | [#92](https://github.com/rluders/amaru/issues/92) | [#18](https://github.com/rluders/amaru/issues/18) |
| FEAT-OBS-001 | M3 | [#170](https://github.com/rluders/amaru/issues/170) | [#18](https://github.com/rluders/amaru/issues/18) |
| FEAT-OPS-002 | M3 | [#177](https://github.com/rluders/amaru/issues/177) | [#14](https://github.com/rluders/amaru/issues/14) |
| FEAT-OPS-003 | M3 | [#178](https://github.com/rluders/amaru/issues/178) | [#19](https://github.com/rluders/amaru/issues/19) |
| FEAT-POL-001 | M3 | [#132](https://github.com/rluders/amaru/issues/132) | [#16](https://github.com/rluders/amaru/issues/16) |
| FEAT-POL-002 | M3 | [#187](https://github.com/rluders/amaru/issues/187) | [#16](https://github.com/rluders/amaru/issues/16) |
| FEAT-POL-003 | M3 | [#188](https://github.com/rluders/amaru/issues/188) | [#16](https://github.com/rluders/amaru/issues/16) |
| FEAT-POL-004 | M3 | [#168](https://github.com/rluders/amaru/issues/168) | [#16](https://github.com/rluders/amaru/issues/16) |
| FEAT-POL-005 | M3 | [#189](https://github.com/rluders/amaru/issues/189) | [#17](https://github.com/rluders/amaru/issues/17) |
| FEAT-POL-006 | M3 | [#190](https://github.com/rluders/amaru/issues/190) | [#17](https://github.com/rluders/amaru/issues/17) |
| FEAT-POL-007 | M3 | [#163](https://github.com/rluders/amaru/issues/163) | [#17](https://github.com/rluders/amaru/issues/17) |
| FEAT-PRJ-001 | M3 | [#120](https://github.com/rluders/amaru/issues/120) | [#18](https://github.com/rluders/amaru/issues/18) |
| FEAT-PRJ-002 | M3 | [#191](https://github.com/rluders/amaru/issues/191) | [#18](https://github.com/rluders/amaru/issues/18) |
| FEAT-PRJ-003 | M3 | [#121](https://github.com/rluders/amaru/issues/121) | [#16](https://github.com/rluders/amaru/issues/16) |
| FEAT-PRJ-005 | M3 | [#193](https://github.com/rluders/amaru/issues/193) | [#14](https://github.com/rluders/amaru/issues/14) |
| FEAT-REV-001 | M3 | [#195](https://github.com/rluders/amaru/issues/195) | [#16](https://github.com/rluders/amaru/issues/16) |
| FEAT-REV-003 | M3 | [#197](https://github.com/rluders/amaru/issues/197) | [#16](https://github.com/rluders/amaru/issues/16) |
| FEAT-SKL-001 | M3 | [#122](https://github.com/rluders/amaru/issues/122) | [#17](https://github.com/rluders/amaru/issues/17) |
| FEAT-SKL-002 | M3 | [#123](https://github.com/rluders/amaru/issues/123) | [#17](https://github.com/rluders/amaru/issues/17) |
| FEAT-WFL-001 | M3 | [#138](https://github.com/rluders/amaru/issues/138) | [#15](https://github.com/rluders/amaru/issues/15) |
| FEAT-WFL-002 | M3 | [#210](https://github.com/rluders/amaru/issues/210) | [#15](https://github.com/rluders/amaru/issues/15) |
| FEAT-WFL-003 | M3 | [#211](https://github.com/rluders/amaru/issues/211) | [#15](https://github.com/rluders/amaru/issues/15) |
| FEAT-WFL-004 | M3 | [#212](https://github.com/rluders/amaru/issues/212) | [#15](https://github.com/rluders/amaru/issues/15) |
| FEAT-WRK-001 | M3 | [#166](https://github.com/rluders/amaru/issues/166) | [#13](https://github.com/rluders/amaru/issues/13) |
| FEAT-WRK-002 | M3 | [#218](https://github.com/rluders/amaru/issues/218) | [#13](https://github.com/rluders/amaru/issues/13) |
| FEAT-WRK-003 | M3 | [#219](https://github.com/rluders/amaru/issues/219) | [#18](https://github.com/rluders/amaru/issues/18) |
| FEAT-ADM-001 | M4 | [#107](https://github.com/rluders/amaru/issues/107) | [#22](https://github.com/rluders/amaru/issues/22) |
| FEAT-CLI-001 | M4 | [#93](https://github.com/rluders/amaru/issues/93) | [#21](https://github.com/rluders/amaru/issues/21) |
| FEAT-CLI-002 | M4 | [#134](https://github.com/rluders/amaru/issues/134) | [#21](https://github.com/rluders/amaru/issues/21) |
| FEAT-CLI-003 | M4 | [#136](https://github.com/rluders/amaru/issues/136) | [#22](https://github.com/rluders/amaru/issues/22) |
| FEAT-CLI-004 | M4 | [#137](https://github.com/rluders/amaru/issues/137) | [#21](https://github.com/rluders/amaru/issues/21) |
| FEAT-CLI-005 | M4 | [#139](https://github.com/rluders/amaru/issues/139) | [#22](https://github.com/rluders/amaru/issues/22) |
| FEAT-CLI-006 | M4 | [#140](https://github.com/rluders/amaru/issues/140) | [#22](https://github.com/rluders/amaru/issues/22) |
| FEAT-CLI-007 | M4 | [#141](https://github.com/rluders/amaru/issues/141) | [#21](https://github.com/rluders/amaru/issues/21) |
| FEAT-INT-001 | M4 | [#164](https://github.com/rluders/amaru/issues/164) | [#25](https://github.com/rluders/amaru/issues/25) |
| FEAT-INT-002 | M4 | [#165](https://github.com/rluders/amaru/issues/165) | [#25](https://github.com/rluders/amaru/issues/25) |
| FEAT-OBS-002 | M4 | [#171](https://github.com/rluders/amaru/issues/171) | [#26](https://github.com/rluders/amaru/issues/26) |
| FEAT-OBS-003 | M4 | [#172](https://github.com/rluders/amaru/issues/172) | [#22](https://github.com/rluders/amaru/issues/22) |
| FEAT-OPS-001 | M4 | [#176](https://github.com/rluders/amaru/issues/176) | [#27](https://github.com/rluders/amaru/issues/27) |
| FEAT-PRJ-004 | M4 | [#192](https://github.com/rluders/amaru/issues/192) | [#22](https://github.com/rluders/amaru/issues/22) |
| FEAT-REV-002 | M4 | [#196](https://github.com/rluders/amaru/issues/196) | [#26](https://github.com/rluders/amaru/issues/26) |
| FEAT-REV-004 | M4 | [#199](https://github.com/rluders/amaru/issues/199) | [#24](https://github.com/rluders/amaru/issues/24) |
| FEAT-REV-005 | M4 | [#200](https://github.com/rluders/amaru/issues/200) | [#25](https://github.com/rluders/amaru/issues/25) |
| FEAT-REV-006 | M4 | [#201](https://github.com/rluders/amaru/issues/201) | [#26](https://github.com/rluders/amaru/issues/26) |
| FEAT-VAL-001 | M4 | [#198](https://github.com/rluders/amaru/issues/198) | [#24](https://github.com/rluders/amaru/issues/24) |
| FEAT-VAL-002 | M4 | [#205](https://github.com/rluders/amaru/issues/205) | [#24](https://github.com/rluders/amaru/issues/24) |
| FEAT-VAL-003 | M4 | [#206](https://github.com/rluders/amaru/issues/206) | [#24](https://github.com/rluders/amaru/issues/24) |
| FEAT-WRK-004 | M4 | [#220](https://github.com/rluders/amaru/issues/220) | [#23](https://github.com/rluders/amaru/issues/23) |
| FEAT-WRK-005 | M4 | [#221](https://github.com/rluders/amaru/issues/221) | [#23](https://github.com/rluders/amaru/issues/23) |
| FEAT-API-004 | M5 | [#98](https://github.com/rluders/amaru/issues/98) | [#31](https://github.com/rluders/amaru/issues/31) |
| FEAT-ART-004 | M5 | [#126](https://github.com/rluders/amaru/issues/126) | [#31](https://github.com/rluders/amaru/issues/31) |
| FEAT-CAP-006 | M5 | [#133](https://github.com/rluders/amaru/issues/133) | [#28](https://github.com/rluders/amaru/issues/28) |
| FEAT-ENV-002 | M5 | [#100](https://github.com/rluders/amaru/issues/100) | [#31](https://github.com/rluders/amaru/issues/31) |
| FEAT-ENV-003 | M5 | [#146](https://github.com/rluders/amaru/issues/146) | [#32](https://github.com/rluders/amaru/issues/32) |
| FEAT-ENV-004 | M5 | [#101](https://github.com/rluders/amaru/issues/101) | [#33](https://github.com/rluders/amaru/issues/33) |
| FEAT-ENV-005 | M5 | [#147](https://github.com/rluders/amaru/issues/147) | [#32](https://github.com/rluders/amaru/issues/32) |
| FEAT-ENV-006 | M5 | [#148](https://github.com/rluders/amaru/issues/148) | [#31](https://github.com/rluders/amaru/issues/31) |
| FEAT-ENV-007 | M5 | [#149](https://github.com/rluders/amaru/issues/149) | [#31](https://github.com/rluders/amaru/issues/31) |
| FEAT-ENV-008 | M5 | [#102](https://github.com/rluders/amaru/issues/102) | [#29](https://github.com/rluders/amaru/issues/29) |
| FEAT-ENV-009 | M5 | [#153](https://github.com/rluders/amaru/issues/153) | [#28](https://github.com/rluders/amaru/issues/28) |
| FEAT-ENV-011 | M5 | [#150](https://github.com/rluders/amaru/issues/150) | [#28](https://github.com/rluders/amaru/issues/28) |
| FEAT-ENV-013 | M5 | [#151](https://github.com/rluders/amaru/issues/151) | [#28](https://github.com/rluders/amaru/issues/28) |
| FEAT-EXE-007 | M5 | [#159](https://github.com/rluders/amaru/issues/159) | [#34](https://github.com/rluders/amaru/issues/34) |
| FEAT-EXE-008 | M5 | [#160](https://github.com/rluders/amaru/issues/160) | [#34](https://github.com/rluders/amaru/issues/34) |
| FEAT-IAM-006 | M5 | [#99](https://github.com/rluders/amaru/issues/99) | [#31](https://github.com/rluders/amaru/issues/31) |
| FEAT-SKL-004 | M5 | [#204](https://github.com/rluders/amaru/issues/204) | [#222](https://github.com/rluders/amaru/issues/222) |
| FEAT-VAL-004 | M5 | [#207](https://github.com/rluders/amaru/issues/207) | [#29](https://github.com/rluders/amaru/issues/29) |
| FEAT-VAL-005 | M5 | [#208](https://github.com/rluders/amaru/issues/208) | [#29](https://github.com/rluders/amaru/issues/29) |
| FEAT-VAL-006 | M5 | [#209](https://github.com/rluders/amaru/issues/209) | [#222](https://github.com/rluders/amaru/issues/222) |
| FEAT-WFL-005 | M5 | [#213](https://github.com/rluders/amaru/issues/213) | [#34](https://github.com/rluders/amaru/issues/34) |
| FEAT-WFL-006 | M5 | [#214](https://github.com/rluders/amaru/issues/214) | [#34](https://github.com/rluders/amaru/issues/34) |
| FEAT-PLG-001 | M6 | [#94](https://github.com/rluders/amaru/issues/94) | [#36](https://github.com/rluders/amaru/issues/36) |
| FEAT-PLG-002 | M6 | [#182](https://github.com/rluders/amaru/issues/182) | [#37](https://github.com/rluders/amaru/issues/37) |
| FEAT-PLG-003 | M6 | covered by [#38](https://github.com/rluders/amaru/issues/38) (`E6.3`) | — |
| FEAT-PLG-004 | M6 | covered by [#39](https://github.com/rluders/amaru/issues/39) (`E6.4`) | — |
| FEAT-PLG-005 | M6 | [#183](https://github.com/rluders/amaru/issues/183) | [#40](https://github.com/rluders/amaru/issues/40) |
| FEAT-PLG-006 | M6 | [#184](https://github.com/rluders/amaru/issues/184) | [#30](https://github.com/rluders/amaru/issues/30) |
| FEAT-PLG-007 | M6 | [#185](https://github.com/rluders/amaru/issues/185) | [#36](https://github.com/rluders/amaru/issues/36) |
| FEAT-PLG-008 | M6 | [#186](https://github.com/rluders/amaru/issues/186) | [#37](https://github.com/rluders/amaru/issues/37) |
| FEAT-ADM-002 | M7 | [#108](https://github.com/rluders/amaru/issues/108) | [#43](https://github.com/rluders/amaru/issues/43) |
| FEAT-ADM-003 | M7 | [#109](https://github.com/rluders/amaru/issues/109) | [#43](https://github.com/rluders/amaru/issues/43) |
| FEAT-AGT-005 | M7 | [#116](https://github.com/rluders/amaru/issues/116) | [#42](https://github.com/rluders/amaru/issues/42) |
| FEAT-CLI-012 | M7 | covered by [#47](https://github.com/rluders/amaru/issues/47) (`E7.6`) | — |
| FEAT-ENV-014 | M7 | [#156](https://github.com/rluders/amaru/issues/156) | [#49](https://github.com/rluders/amaru/issues/49) |
| FEAT-IAM-002 | M7 | [#96](https://github.com/rluders/amaru/issues/96) | [#43](https://github.com/rluders/amaru/issues/43) |
| FEAT-IAM-003 | M7 | [#97](https://github.com/rluders/amaru/issues/97) | [#43](https://github.com/rluders/amaru/issues/43) |
| FEAT-IAM-004 | M7 | [#115](https://github.com/rluders/amaru/issues/115) | [#43](https://github.com/rluders/amaru/issues/43) |
| FEAT-IAM-005 | M7 | [#161](https://github.com/rluders/amaru/issues/161) | [#43](https://github.com/rluders/amaru/issues/43) |
| FEAT-INT-003 | M7 | [#167](https://github.com/rluders/amaru/issues/167) | [#42](https://github.com/rluders/amaru/issues/42) |
| FEAT-INT-004 | M7 | [#169](https://github.com/rluders/amaru/issues/169) | [#42](https://github.com/rluders/amaru/issues/42) |
| FEAT-OBS-004 | M7 | [#173](https://github.com/rluders/amaru/issues/173) | [#46](https://github.com/rluders/amaru/issues/46) |
| FEAT-OBS-005 | M7 | [#174](https://github.com/rluders/amaru/issues/174) | [#46](https://github.com/rluders/amaru/issues/46) |
| FEAT-OBS-006 | M7 | [#175](https://github.com/rluders/amaru/issues/175) | [#46](https://github.com/rluders/amaru/issues/46) |
| FEAT-OPS-004 | M7 | [#179](https://github.com/rluders/amaru/issues/179) | [#45](https://github.com/rluders/amaru/issues/45) |
| FEAT-OPS-005 | M7 | [#180](https://github.com/rluders/amaru/issues/180) | [#45](https://github.com/rluders/amaru/issues/45) |
| FEAT-OPS-006 | M7 | [#181](https://github.com/rluders/amaru/issues/181) | [#45](https://github.com/rluders/amaru/issues/45) |
| FEAT-PLG-009 | M7 | [#155](https://github.com/rluders/amaru/issues/155) | [#49](https://github.com/rluders/amaru/issues/49) |
| FEAT-POL-008 | M7 | covered by [#48](https://github.com/rluders/amaru/issues/48) (`E7.7`) | — |
| FEAT-PRJ-006 | M7 | [#194](https://github.com/rluders/amaru/issues/194) | [#43](https://github.com/rluders/amaru/issues/43) |
| FEAT-REV-007 | M7 | [#202](https://github.com/rluders/amaru/issues/202) | [#42](https://github.com/rluders/amaru/issues/42) |
| FEAT-SKL-003 | M7 | [#203](https://github.com/rluders/amaru/issues/203) | [#42](https://github.com/rluders/amaru/issues/42) |
| FEAT-WFL-007 | M7 | [#215](https://github.com/rluders/amaru/issues/215) | [#42](https://github.com/rluders/amaru/issues/42) |
| FEAT-WFL-008 | M7 | [#216](https://github.com/rluders/amaru/issues/216) | [#42](https://github.com/rluders/amaru/issues/42) |
| FEAT-WFL-009 | M7 | [#217](https://github.com/rluders/amaru/issues/217) | [#42](https://github.com/rluders/amaru/issues/42) |
| FEAT-CLI-008 | M8 | covered by [#51](https://github.com/rluders/amaru/issues/51) (`E8.2`) | — |
| FEAT-CLI-009 | M8 | [#142](https://github.com/rluders/amaru/issues/142) | [#53](https://github.com/rluders/amaru/issues/53) |
| FEAT-CLI-010 | M8 | [#143](https://github.com/rluders/amaru/issues/143) | [#52](https://github.com/rluders/amaru/issues/52) |
| FEAT-CLI-011 | M8 | [#144](https://github.com/rluders/amaru/issues/144) | [#53](https://github.com/rluders/amaru/issues/53) |
| FEAT-IAM-007 | M8 | [#162](https://github.com/rluders/amaru/issues/162) | [#52](https://github.com/rluders/amaru/issues/52) |
| FEAT-ENV-012 | M9 | covered by [#59](https://github.com/rluders/amaru/issues/59) (`E9.3`) | — |
| FEAT-OPS-007 | M9 | covered by [#57](https://github.com/rluders/amaru/issues/57) (`E9.2`) | — |
| FEAT-OPS-008 | M9 | covered by [#58](https://github.com/rluders/amaru/issues/58) (`E9.4`) | — |
| FEAT-POL-009 | M9 | covered by [#56](https://github.com/rluders/amaru/issues/56) (`E9.1`) | — |
| FEAT-PLG-010 | M10 | [#63](https://github.com/rluders/amaru/issues/63) | — |

## 3. Feature-to-epic mapping (M5–M9)

Confirmed in the backlog refinement of 2026-10-25 against the roadmap exit criteria, catalog dependencies and Work Package boundaries; the [roadmap](../architecture/ROADMAP.md) lists the same grouping per epic. Each scheduled M5–M9 feature has exactly one owning epic. No milestone or priority changed.

| Epic | Features | Change from the initial proposal |
|---|---|---|
| `E5.1` [#31](https://github.com/rluders/amaru/issues/31) | FEAT-API-004, FEAT-ART-004 (port and filesystem adapter), FEAT-ENV-002, FEAT-ENV-006, FEAT-ENV-007, FEAT-IAM-006 | ART-004 and ENV-007 moved from E5.3: Worker uploads are part of the protocol data plane and need the store port |
| `E5.2` [#32](https://github.com/rluders/amaru/issues/32) | FEAT-ENV-003, FEAT-ENV-005 | — |
| `E5.3` [#33](https://github.com/rluders/amaru/issues/33) | FEAT-ENV-004 | Leases plus the partition suite of the M5 exit |
| `E5.4` [#29](https://github.com/rluders/amaru/issues/29) | FEAT-ENV-008, FEAT-VAL-004, FEAT-VAL-005 | — |
| `E5.5` [#28](https://github.com/rluders/amaru/issues/28) | FEAT-CAP-006, FEAT-ENV-009, FEAT-ENV-011, FEAT-ENV-013 | — |
| `E5.6` [#34](https://github.com/rluders/amaru/issues/34) | FEAT-EXE-007, FEAT-EXE-008, FEAT-WFL-005, FEAT-WFL-006 | SKL-004 and VAL-006 moved to E5.7 |
| `E5.7` [#222](https://github.com/rluders/amaru/issues/222) | FEAT-SKL-004, FEAT-VAL-006 | New epic: delivery-flow efficiency fits none of E5.1–E5.6 |
| `E6.1` [#36](https://github.com/rluders/amaru/issues/36) | FEAT-PLG-001, FEAT-PLG-007 | — |
| `E6.2` [#37](https://github.com/rluders/amaru/issues/37) | FEAT-PLG-002, FEAT-PLG-008 | — |
| `E6.3` [#38](https://github.com/rluders/amaru/issues/38) | FEAT-PLG-003 | — |
| `E6.4` [#39](https://github.com/rluders/amaru/issues/39) | FEAT-PLG-004 | — |
| `E6.5` [#40](https://github.com/rluders/amaru/issues/40) | FEAT-PLG-005 | — |
| `E6.6` [#30](https://github.com/rluders/amaru/issues/30) | FEAT-PLG-006 | — |
| `E7.1` [#43](https://github.com/rluders/amaru/issues/43) | FEAT-ADM-002, FEAT-ADM-003, FEAT-IAM-002, FEAT-IAM-003, FEAT-IAM-004, FEAT-IAM-005, FEAT-PRJ-006 | PRJ-006 moved from E7.3: Project archive is Organization data administration beside ADM-003, not packaging |
| `E7.2` [#44](https://github.com/rluders/amaru/issues/44) | — (roadmap scope) plus the S3-compatible adapter of FEAT-ART-004 | ART-004 is owned by E5.1 for M5; E7.2 delivers its M7 part |
| `E7.3` [#45](https://github.com/rluders/amaru/issues/45) | FEAT-OPS-004, FEAT-OPS-005, FEAT-OPS-006 | — |
| `E7.4` [#42](https://github.com/rluders/amaru/issues/42) | FEAT-AGT-005, FEAT-INT-003, FEAT-INT-004, FEAT-REV-007, FEAT-SKL-003, FEAT-WFL-007, FEAT-WFL-008, FEAT-WFL-009 | — |
| `E7.5` [#46](https://github.com/rluders/amaru/issues/46) | FEAT-OBS-004, FEAT-OBS-005, FEAT-OBS-006 | — |
| `E7.6` [#47](https://github.com/rluders/amaru/issues/47) | FEAT-CLI-012 | — |
| `E7.7` [#48](https://github.com/rluders/amaru/issues/48) | FEAT-POL-008 | `SECURITY.md` is created early by the M1 owner task [#71](https://github.com/rluders/amaru/issues/71); E7.7 verifies it for the self-hosted release |
| `E7.8` [#49](https://github.com/rluders/amaru/issues/49) | FEAT-ENV-014, FEAT-PLG-009 | — |
| `E8.1` [#50](https://github.com/rluders/amaru/issues/50) | — (roadmap scope only) | — |
| `E8.2` [#51](https://github.com/rluders/amaru/issues/51) | FEAT-CLI-008 | — |
| `E8.3` [#52](https://github.com/rluders/amaru/issues/52) | FEAT-CLI-010, FEAT-IAM-007 | — |
| `E8.4` [#53](https://github.com/rluders/amaru/issues/53) | FEAT-CLI-009, FEAT-CLI-011 | — |
| `E8.5` [#54](https://github.com/rluders/amaru/issues/54) | — (roadmap scope only) | — |
| `E9.1` [#56](https://github.com/rluders/amaru/issues/56) | FEAT-POL-009 | — |
| `E9.2` [#57](https://github.com/rluders/amaru/issues/57) | FEAT-OPS-007 | — |
| `E9.3` [#59](https://github.com/rluders/amaru/issues/59) | FEAT-ENV-012 | — |
| `E9.4` [#58](https://github.com/rluders/amaru/issues/58) | FEAT-OPS-008 | — |

## 4. Dependency notes and gates

- **Cycle FEAT-CAP-002 ↔ FEAT-POL-006: corrected.** POL-006 depends on CAP-002 and AGT-003 ([#190](https://github.com/rluders/amaru/issues/190) blocked by [#127](https://github.com/rluders/amaru/issues/127) and [#113](https://github.com/rluders/amaru/issues/113)); CAP-002 no longer depends on POL-006.
- **Cross-package dependencies** (resolutions in [WORK_PACKAGES](../architecture/WORK_PACKAGES.md#cross-package-feature-dependencies)):
  - FEAT-ART-003 → FEAT-ART-002: hard. ART-003 [#125](https://github.com/rluders/amaru/issues/125) moved from WP-M3-02 [#14](https://github.com/rluders/amaru/issues/14) to WP-M3-07 [#19](https://github.com/rluders/amaru/issues/19) and is blocked by REV-003 [#197](https://github.com/rluders/amaru/issues/197), which is blocked by ART-002 [#124](https://github.com/rluders/amaru/issues/124).
  - FEAT-WRK-001 → FEAT-PRJ-001: soft, recorded in [#166](https://github.com/rluders/amaru/issues/166).
  - FEAT-WRK-004 → FEAT-VAL-001, FEAT-REV-004, FEAT-REV-005, FEAT-INT-001: soft, recorded in [#220](https://github.com/rluders/amaru/issues/220); the real run is in WP-M4-07 [#27](https://github.com/rluders/amaru/issues/27).
- **Human approval gates** ([delivery §8](delivery.md#8-human-approval-gates)): all `MADR-*` issues and [#65](https://github.com/rluders/amaru/issues/65), [#66](https://github.com/rluders/amaru/issues/66), [#69](https://github.com/rluders/amaru/issues/69) carry `awaiting-owner-approval`. Every migration ADR issue is blocked by [#8](https://github.com/rluders/amaru/issues/8) (inventory refresh).
- **Decided 2026-10-26 (product owner):** the license state is the one decided in ADR-0026 and stated in the README. No separate legal review: [#65](https://github.com/rluders/amaru/issues/65) closed. The root `LICENSE` [#66](https://github.com/rluders/amaru/issues/66) no longer blocks migration ADR issues and is no longer a sub-issue of WP-M1-01 [#2](https://github.com/rluders/amaru/issues/2) or an M1 exit criterion; it stays open, owned by the product owner, before the first public release or external contribution.

- **Decided 2026-10-25 (delegated by the product owner):** the epic changes of §3, ART-003's move and its REV-003 dependency, and the WP-M1-01 *Done when* correction are confirmed. Default OS owner for M3/M4 is the product owner. The Go Worker SDK is created in E6.6 [#30](https://github.com/rluders/amaru/issues/30). The egress check that POL-004 [#168](https://github.com/rluders/amaru/issues/168) reuses from `transport/webhook/sender.go` is covered by the M3 migration ADR MADR-httpapi [#83](https://github.com/rluders/amaru/issues/83), which blocks #168; the rest of `transport/webhook` stays with MADR-webhook [#95](https://github.com/rluders/amaru/issues/95) in M7. License decisions are deferred by the product owner; their gates stay open.

## 5. Not tracked as issues

- P2 features in the catalog *Backlog* and all P3 features: not scheduled, so no issue (catalog priority rules).
