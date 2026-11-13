---
title: Subscribe to coordinator wakes before reading queued work
author: Ricardo
date: 2026-11-13
branch: fix/191-coordinator-wake
id: l-cde73c9e
scope: repo
status: active
src: agent
touches:
  - services/platform/internal/coordinator/serve.go
  - services/platform/internal/coordinator/serve_test.go
---

## Decided
In services/platform/internal/coordinator/serve.go, capture c.wake() before attempting to lease queued work, and select on that captured channel. Wake closes and replaces the shared channel: capturing it after an empty queue snapshot loses submissions committed in between. The deterministic TestServeWakeDuringEmptyQueueRead in serve_test.go commits through real Coordinator.Submit inside a fake queue-read callback, then returns the earlier empty snapshot. It fails on unchanged b490ed6 and passes with early subscription, without a second Offer, heartbeat, or Wake.

## Tried and ruled out
One hundred passing race repetitions of the independent-Project test during PR #352 did not disprove this scheduling window. Increasing its timeout or changing Worker heartbeat frequency would hide the bug. No production test hooks or changes to the Worker protocol are needed: control the snapshot boundary through the existing ExecutionStore port.

## Watch out
The earlier PR #352 provider-failure timeout is consistent with this race but was not causally reproduced. Do not claim that exact timeout's cause is proven. Preserve broadcast semantics for multiple Worker sessions, lease commits before sends, and session cancellation/error classification from the Worker shutdown learning. This fixes single-Platform scheduling; it does not add distributed scheduling authority.
