-- +goose Up
-- Internal result receipts and durable, reference-only publication intent. This
-- is not the public Event log: no cursor or wire envelope is assigned here.
CREATE TABLE result_captures (
 attempt_id text PRIMARY KEY,
 execution_id text NOT NULL REFERENCES executions(id),
 step_id text NOT NULL REFERENCES step_executions(id),
 captured_at timestamptz NOT NULL,
 receipt jsonb NOT NULL
);
CREATE INDEX result_captures_execution ON result_captures(execution_id, captured_at, attempt_id);
CREATE TABLE usage_publication_intents (
 usage_id text PRIMARY KEY,
 attempt_id text NOT NULL REFERENCES result_captures(attempt_id),
 ordinal integer NOT NULL CHECK (ordinal >= 0),
 UNIQUE(attempt_id, ordinal)
);

-- +goose StatementBegin
CREATE FUNCTION result_captures_are_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'a result receipt is immutable';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER result_captures_immutable BEFORE UPDATE OR DELETE ON result_captures
    FOR EACH ROW EXECUTE FUNCTION result_captures_are_immutable();

-- +goose Down
DROP TABLE usage_publication_intents;
DROP TABLE result_captures;
DROP FUNCTION result_captures_are_immutable();
