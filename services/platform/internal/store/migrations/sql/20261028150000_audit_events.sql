-- +goose Up
-- The audit log of approvals, grants and policy changes (FEAT-POL-003). One log for the whole
-- Platform; seq has no gaps. audit_counter is the single row that hands out seq: a writer locks it
-- for its transaction, so numbers are assigned in commit order and a reader that resumes after the
-- last seq it saw misses nothing. A sequence would leave gaps and let a later number commit first.
-- Rows are never updated or deleted; the trigger refuses both.
CREATE TABLE audit_counter (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    last_seq  bigint  NOT NULL CHECK (last_seq >= 0)
);
INSERT INTO audit_counter (singleton, last_seq) VALUES (true, 0);

CREATE TABLE audit_events (
    seq         bigint      PRIMARY KEY CHECK (seq >= 1),
    at          timestamptz NOT NULL,
    kind        text        NOT NULL CHECK (kind <> ''),
    actor       text        NOT NULL CHECK (actor <> ''),
    actor_kind  text        NOT NULL CHECK (actor_kind IN ('human', 'platform')),
    target_type text        NOT NULL CHECK (target_type <> ''),
    target_id   text        NOT NULL CHECK (target_id <> ''),
    project_id  text        NOT NULL DEFAULT '',
    reason      text        NOT NULL DEFAULT ''
);
CREATE INDEX audit_events_target ON audit_events (target_type, target_id, seq);

-- +goose StatementBegin
CREATE FUNCTION audit_events_are_append_only() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit event % is append-only: it cannot be %', OLD.seq, lower(TG_OP);
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER audit_events_append_only BEFORE UPDATE OR DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION audit_events_are_append_only();
