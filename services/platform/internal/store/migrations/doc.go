// Package migrations applies the Platform's versioned, forward-only PostgreSQL migrations
// (ADR-0015, ADR-0023, FEAT-OPS-002).
//
// The migration directory is embedded in the binary: goose-format SQL files named
// <YYYYMMDDHHMMSS>_<name>.sql plus migrations.sum, a sha256 manifest of those files. The
// package uses goose (github.com/pressly/goose/v3) as a library to run each file in its own
// transaction together with its version row.
//
// Rules the package enforces:
//   - the directory must match migrations.sum, so an edited, added or removed file is refused
//     before anything runs (ErrChecksum);
//   - a database that records a version the build does not know is newer than the build; it is
//     refused, never downgraded (ErrSchemaNewer);
//   - a file older than an applied one is refused (ErrOutOfOrder);
//   - one runner at a time: an advisory lock serializes concurrent Platforms.
//
// goose keeps no file hash, so a file edited after it was applied is caught only by
// migrations.sum at build time (go test), not against a live database.
//
// This package is an adapter. The core (internal/domain) never imports it.
package migrations
