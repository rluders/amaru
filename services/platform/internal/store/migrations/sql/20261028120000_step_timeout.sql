-- +goose Up
-- The wall-clock timeout override of a Step in seconds (FEAT-WFL-004); zero means the default for its kind.
ALTER TABLE step_executions
    ADD COLUMN timeout_seconds integer NOT NULL DEFAULT 0 CHECK (timeout_seconds >= 0);
