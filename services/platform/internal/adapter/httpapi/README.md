# Project settings HTTP adapter

Owned by WP-M3-06 (#18), Project isolation #191. `SettingsHandler` serves the existing Project settings GET/PATCH resource through compact authentication and the authorized application boundary. [ADR-0059](../../../../../docs/architecture/adr/0059-adapt-the-prototype-http-handlers-to-the-platform-public-api.md) permits fresh handlers where Amaru contracts differ; this code is written fresh, with no predecessor material copied. ADR-0062 and ADR-0064 define access and preference semantics.

Trusted composition supplies one Auth instance, ProjectAccess using its Principal callback, and the same Instance's SettingsStore. The returned handler includes authentication; do not wrap it in another Auth instance. Production startup and other resources are not implemented here. Settings PATCH never reads or writes Project files, starts a Worker, grants trust or performs Skill discovery.

From the repository root, `make platform-test` runs these tests and `make check` runs the full checks. For focused race verification, from `services/platform`, run `go test -race -count=10 ./internal/adapter/httpapi`. Tests use fake stores, real compact authentication and a temporary loopback listener; no paid APIs or real agents. The generated SDK also exercises these handlers in the test-only auth fixture via `make sdk-ts`; actual browser SDK and full product acceptance remain separate.

See the [operation inventory and limits](../../../../../docs/engineering/project-access.md#guarded-settings-http-adapter).
