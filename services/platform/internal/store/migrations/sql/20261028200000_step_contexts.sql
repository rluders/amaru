-- +goose Up
-- The context assembled for a Step (FEAT-SKL-002): the exact document an Agent was told, its hash
-- and the list of items that went into it. It is Evidence, so it is written once: the trigger
-- refuses an update and a delete. The database checks that the hash is the SHA-256 of the
-- document, so a row cannot name a document it does not hold.
CREATE TABLE step_contexts (
    execution_id text        NOT NULL,
    step_id      text        NOT NULL REFERENCES step_executions (id),
    format       text        NOT NULL CHECK (format <> ''),
    hash         text        NOT NULL,
    document     text        NOT NULL CHECK (document <> '' AND octet_length(document) <= 4194304),
    items        jsonb       NOT NULL,
    created_at   timestamptz NOT NULL,
    PRIMARY KEY (execution_id, step_id),
    CHECK (hash = 'sha256:' || encode(sha256(convert_to(document, 'UTF8')), 'hex'))
);

-- +goose StatementBegin
CREATE FUNCTION step_contexts_are_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'the context of step % is immutable: it cannot be %', OLD.step_id, lower(TG_OP);
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER step_contexts_immutable BEFORE UPDATE OR DELETE ON step_contexts
    FOR EACH ROW EXECUTE FUNCTION step_contexts_are_immutable();
