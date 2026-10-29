---
title: Amaru Project API writes need readback and bounded batches
author: Ricardo
date: 2026-10-29
branch: docs/github-project-management
id: l-af934980
scope: repo
status: active
src: agent
touches:
  - docs/engineering/delivery.md
---

## Decided
Use API readback, not endpoint availability, to decide which GitHub features work for Amaru. In this personal repository the issue-types endpoint lists Task/Bug/Feature, but updateIssueIssueType returns issueType:null and the value stays null on a fresh query. docs/engineering/delivery.md therefore keeps the existing planning-role labels as the classification source of truth.

## Tried and ruled out
GraphQL view updates cannot configure groups; the supported REST view-creation endpoint can. REST rejected the native Updated timestamp field ID even though GraphQL accepted it as a visible column, so configure supported groups through REST and timestamp visibility through GraphQL. Do not infer native type support or REST field support from a listing alone.

## Watch out
Large GraphQL mutation batches produced HTTP 502 responses. Project imports also returned internal errors while the default automatic sub-issue expansion was adding children; some writes persisted despite the error. Disable redundant sub-issue expansion for an explicit full import, query existing item IDs before retrying, use small idempotent batches, and re-query the final state. The validation record names the verified configuration and remaining UI limits; an API error is not proof that nothing changed.
