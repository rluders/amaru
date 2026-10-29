// Package osproc is the Worker OS layer (ADR-0053, FEAT-EXE-003): it starts local programs and
// ends their whole process tree. It is the only place in the Platform module that starts a
// process. Providers reach it through ports.ProcessRunner and ports.ExecutableResolver; the core,
// the coordinator and the embedded Worker import it nowhere (internal/arch enforces it).
//
// Adapted from the earlier internal implementation's platform package at commit c8a2f6e
// (ADR-0032); see PROVENANCE.md.
//
// Write confinement (ADR-0057): a ProcSpec with Confine set is started through a launch helper,
// the Worker's own binary, whose main must call ConfineExecMain first. On Linux the helper applies
// a Landlock ruleset and then execs the program; elsewhere, or when Landlock is missing or too
// old, the start is refused rather than run unconfined.
//
// Platform support: Linux is tested. macOS shares the Unix code and is best-effort. Windows uses
// taskkill and is best-effort and not validated (ADR-0053 item 5): it may leave a descendant that
// outlived its parent.
package osproc
