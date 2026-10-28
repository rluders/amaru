-- +goose Up
-- A Step carries its Capability and resolved inputs, and each Lease is one attempt with a fencing
-- token (ADR-0044 point 4). attempt_id and fence_token outlive the Lease so that a late result can
-- be told from a duplicate and from a stale one.
ALTER TABLE step_executions
    ADD COLUMN capability  text    NOT NULL DEFAULT '',
    ADD COLUMN inputs      bytea,
    ADD COLUMN attempt_id  text,
    ADD COLUMN fence_token bigint  NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX step_executions_attempt ON step_executions (attempt_id) WHERE attempt_id IS NOT NULL;
-- The coordinator hands out queued Steps oldest first.
CREATE INDEX step_executions_queued ON step_executions (execution_id, ord) WHERE state = 'queued';
