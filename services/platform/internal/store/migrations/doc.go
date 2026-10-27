// Package migrations applies the Platform's versioned, forward-only PostgreSQL migrations
// (ADR-0015, ADR-0023, FEAT-OPS-002).
//
// The migration directory is embedded in the binary: goose-format SQL files authored with
// Atlas, plus Atlas' atlas.sum integrity file. The package uses Atlas as a library
// (ariga.io/atlas/sql/migrate) to read the directory and execute files, and keeps its own
// revisions table so each file and its revision row commit in one transaction.
//
// Rules the package enforces:
//   - the directory must match atlas.sum, so an edited or reordered file is refused;
//   - a database whose recorded versions the build does not know is newer than the build;
//     it is refused, never downgraded (ErrSchemaNewer);
//   - an already applied file whose content changed is refused (ErrHistoryChanged);
//   - one runner at a time: an advisory lock serializes concurrent Platforms.
//
// This package is an adapter. The core (internal/domain) never imports it.
package migrations
