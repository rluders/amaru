-- +goose Up
-- Execution state is durable (ADR-0042). A change to an Execution, its Steps and its event log is
-- committed in one transaction, so a transition is on disk before anyone is told about it.
-- version is the optimistic-concurrency counter: a write succeeds only against the version it read.
CREATE TABLE executions (
    id            text        PRIMARY KEY,
    version       bigint      NOT NULL,
    state         text        NOT NULL CHECK (state IN ('queued', 'running', 'waiting', 'succeeded', 'failed', 'cancelled')),
    created_at    timestamptz NOT NULL,
    started_at    timestamptz,
    ended_at      timestamptz,
    error_code    text,
    error_message text,
    -- last event seq assigned; bumping it under the row lock gives each Execution a gapless log
    next_seq      bigint      NOT NULL DEFAULT 0
);
-- Reconcile at start scans the Executions that are not finished.
CREATE INDEX executions_open ON executions (id) WHERE state NOT IN ('succeeded', 'failed', 'cancelled');

CREATE TABLE step_executions (
    id                   text        PRIMARY KEY,
    execution_id         text        NOT NULL REFERENCES executions (id),
    ord                  integer     NOT NULL,
    version              bigint      NOT NULL,
    state                text        NOT NULL CHECK (state IN ('pending', 'skipped', 'refused', 'awaiting_approval', 'queued',
                                                               'leased', 'running', 'succeeded', 'failed', 'timed_out',
                                                               'interrupted', 'cancelled')),
    reason               text        NOT NULL DEFAULT '',
    started_at           timestamptz,
    ended_at             timestamptz,
    error_code           text,
    error_message        text,
    idempotent           boolean     NOT NULL DEFAULT false,
    remote_job_id        text,
    remote_job_resumable boolean     NOT NULL DEFAULT false,
    lease_owner          text,
    lease_expires_at     timestamptz,
    -- the Agent (and Skills) snapshot the Step runs with; never changed after it is bound
    agent                jsonb,
    UNIQUE (execution_id, ord)
);

CREATE TABLE execution_events (
    execution_id text        NOT NULL REFERENCES executions (id),
    seq          bigint      NOT NULL,
    kind         text        NOT NULL,
    step_id      text        NOT NULL DEFAULT '',
    reason       text        NOT NULL DEFAULT '',
    at           timestamptz NOT NULL,
    PRIMARY KEY (execution_id, seq)
);
