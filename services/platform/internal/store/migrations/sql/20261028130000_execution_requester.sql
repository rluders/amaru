-- +goose Up
-- Who submitted an Execution and how it was started, so Policy is evaluated for them before each
-- Step starts (FEAT-POL-001). Set when the Execution is created; Commit never updates them, and the
-- trigger refuses an update that tries.
ALTER TABLE executions
    ADD COLUMN requested_by jsonb NOT NULL DEFAULT '{}',
    ADD COLUMN trigger_kind text  NOT NULL DEFAULT '';

-- +goose StatementBegin
CREATE FUNCTION executions_requester_is_immutable() RETURNS trigger AS $$
BEGIN
    IF NEW.requested_by IS DISTINCT FROM OLD.requested_by OR NEW.trigger_kind IS DISTINCT FROM OLD.trigger_kind THEN
        RAISE EXCEPTION 'execution % requester is fixed and cannot change', OLD.id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER executions_requester_immutable BEFORE UPDATE ON executions
    FOR EACH ROW EXECUTE FUNCTION executions_requester_is_immutable();
