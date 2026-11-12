-- +goose Up
-- Transactional counters serialize Project writers through commit, unlike SQL sequences.
CREATE TABLE project_event_counters (
    project_id text PRIMARY KEY,
    last_seq bigint NOT NULL DEFAULT 0 CHECK (last_seq >= 0),
    unavailable_through bigint NOT NULL DEFAULT 0 CHECK (unavailable_through >= 0 AND unavailable_through <= last_seq)
);

-- Historical per-Execution logs cannot reconstruct cross-Execution commit order.
-- Reserve a baseline cursor and require a state reload rather than inventing that order.
INSERT INTO project_event_counters (project_id, last_seq, unavailable_through)
SELECT DISTINCT e.project_id, 1, 1 FROM executions e
JOIN execution_events ev ON ev.execution_id = e.id;

CREATE TABLE project_events (
    project_id text NOT NULL REFERENCES project_event_counters(project_id),
    seq bigint NOT NULL CHECK (seq >= 1),
    execution_id text NOT NULL,
    execution_seq bigint NOT NULL,
    recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (project_id, seq),
    UNIQUE (execution_id, execution_seq),
    FOREIGN KEY (execution_id, execution_seq) REFERENCES execution_events(execution_id, seq)
);
CREATE INDEX project_events_expiry ON project_events (project_id, recorded_at);
CREATE INDEX project_events_execution ON project_events (project_id, execution_id, seq);
