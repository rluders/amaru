---
name: testing-and-validation
description: Decide which checks a change in Amaru needs, run them, and report results with honest evidence levels (unit-tested, integration-tested, real-validated, not validated). Use before claiming any change is done, and when adding tests or a new validation target.
---

# Testing and validation

## Purpose
Make "done" mean "checked", and make reports say exactly what was and was not proven.

## When to use
- Before reporting a change as complete.
- When adding code, a component, or a Makefile target.
- When a test fails or is flaky.

## Workflow
1. **Map the change to checks.** Read [validation.md](../../../docs/engineering/validation.md) and the root `Makefile`. Docs-only change → `make docs-check`. Code in a component → that component's targets. Always finish with `make check`.
2. **Reproduce first** when fixing a failure: run the failing check, capture the output, then fix.
3. **Write tests at the right level.**
   - Unit: pure logic with fakes; table-driven; no real agents, paid APIs or specialized tools.
   - Integration: real components on loopback, temp dirs, child processes; must not need network beyond loopback.
   - Real validation: a recorded run against the real tool or service — date, versions, steps, result written into the repo. Not automated in `make check`.
   - Cross-platform behavior needs tests that run on that platform; say which platforms actually ran.
4. **Run** the checks. Use `-count=1` for Go when fixtures outside the module may have changed.
5. **Flakes**: "flaky" is not a root cause. Re-run once to confirm, then find the race or timing assumption. Never skip or disable a test to get green.
6. **Review your own diff** for what CI or a reviewer would reject: formatting, unused code, missing docs updates.

## Output
For each check: the exact command, pass/fail with counts, and anything skipped with the reason (tool missing, platform unavailable, no network). Then one line per behavior: *unit-tested* / *integration-tested* / *real-validated* / *not validated*.

## Verification
- Every "passed" in the report corresponds to a command actually run in this session.
- Local results are never reported as CI results.
- Nothing is called validated because it compiles.
