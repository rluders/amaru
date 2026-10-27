// Package coordinator is the Execution coordinator of the Platform (WP-M3-03, ADR-0042,
// ADR-0052). It drives Executions through ports only: all state lives in ports.ExecutionStore,
// so a restarted process continues from what was committed and from nothing else.
//
// Implemented so far: restart reconciliation (Reconcile). Leasing, Worker reports, timers and the
// outbox arrive with their own issues.
package coordinator
