// Package ports declares the interfaces through which the Platform core reaches storage and, later,
// other adapters. A port depends only on internal/domain. Adapters implement ports and live under
// internal/store; in-memory fakes live in internal/ports/fakes; the shared behavior tests live in
// internal/ports/porttest.
package ports
