-- +goose Up
-- The Project and Environment an Execution runs in, for the concurrency limits (FEAT-EXE-006).
-- They are opaque ids here; empty means "not scoped". They never change after the Execution exists.
ALTER TABLE executions
    ADD COLUMN project_id     text NOT NULL DEFAULT '',
    ADD COLUMN environment_id text NOT NULL DEFAULT '';
-- The coordinator counts the Steps that hold a slot.
CREATE INDEX step_executions_active ON step_executions (execution_id) WHERE state IN ('leased', 'running');
