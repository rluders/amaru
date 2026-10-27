// Package postgres implements the Platform's storage ports on PostgreSQL (ADR-0015, ADR-0050).
//
// It is an adapter: the core never imports it. The schema comes from internal/store/migrations;
// this package assumes the migrations have been applied and never creates or alters tables. It
// touches the database only; it reads and writes no file, so nothing it stores lands in a
// Project tree (ADR-0043).
package postgres
