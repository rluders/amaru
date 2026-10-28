-- +goose Up
-- The Workflow definition an Execution runs, pinned when the Execution is created (FEAT-WFL-002).
-- The snapshot is text, not jsonb, because the hash is over its exact bytes. Commit never updates
-- these columns, and the trigger refuses an update that tries, so the pin is immutable.
ALTER TABLE executions
    ADD COLUMN definition_workflow text,
    ADD COLUMN definition_revision text,
    ADD COLUMN definition_hash     text,
    ADD COLUMN definition_snapshot text,
    ADD CONSTRAINT executions_definition_whole CHECK (
        (definition_workflow IS NULL AND definition_hash IS NULL AND definition_snapshot IS NULL AND definition_revision IS NULL)
        OR (definition_workflow IS NOT NULL AND definition_hash IS NOT NULL AND definition_snapshot IS NOT NULL));

-- +goose StatementBegin
CREATE FUNCTION executions_definition_is_immutable() RETURNS trigger AS $$
BEGIN
    IF NEW.definition_workflow IS DISTINCT FROM OLD.definition_workflow
       OR NEW.definition_revision IS DISTINCT FROM OLD.definition_revision
       OR NEW.definition_hash IS DISTINCT FROM OLD.definition_hash
       OR NEW.definition_snapshot IS DISTINCT FROM OLD.definition_snapshot THEN
        RAISE EXCEPTION 'execution % definition is pinned and cannot change', OLD.id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER executions_definition_immutable BEFORE UPDATE ON executions
    FOR EACH ROW EXECUTE FUNCTION executions_definition_is_immutable();
