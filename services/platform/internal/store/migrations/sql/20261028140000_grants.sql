-- +goose Up
-- Grants: standing human decisions that authorize a Subject to use Capabilities (FEAT-POL-001,
-- ADR-0037). A Grant is never deleted: revoking, lapsing and expiring keep the row for the audit
-- trail. Only the status and the revocation columns change after creation; the trigger refuses any
-- other update, so what a human granted cannot be edited in place.
CREATE TABLE grants (
    id              text        PRIMARY KEY,
    version         bigint      NOT NULL CHECK (version >= 1),
    level           text        NOT NULL CHECK (level IN ('organization', 'project')),
    project_id      text        NOT NULL DEFAULT '',
    subject_kind    text        NOT NULL CHECK (subject_kind IN ('agent', 'workflow', 'plugin_component')),
    subject_id      text        NOT NULL CHECK (subject_id <> ''),
    capabilities    jsonb       NOT NULL CHECK (jsonb_typeof(capabilities) = 'array' AND jsonb_array_length(capabilities) > 0),
    resource_type   text        NOT NULL DEFAULT '',
    resource_value  text        NOT NULL DEFAULT '',
    definition_hash text        NOT NULL DEFAULT '',
    granted_by      text        NOT NULL CHECK (granted_by <> ''),
    created_at      timestamptz NOT NULL,
    expires_at      timestamptz,
    status          text        NOT NULL CHECK (status IN ('active', 'lapsed', 'revoked', 'expired')),
    revoked_by      text        NOT NULL DEFAULT '',
    revoked_at      timestamptz,
    revoke_reason   text        NOT NULL DEFAULT '',
    CONSTRAINT grants_project_level CHECK ((level = 'project') = (project_id <> '')),
    CONSTRAINT grants_revoked_whole CHECK ((status = 'revoked') = (revoked_by <> '' AND revoked_at IS NOT NULL))
);
CREATE INDEX grants_active ON grants (project_id) WHERE status = 'active';

-- +goose StatementBegin
CREATE FUNCTION grants_are_append_only() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'grant % cannot be deleted; revoke it', OLD.id;
    END IF;
    IF NEW.id IS DISTINCT FROM OLD.id OR NEW.level IS DISTINCT FROM OLD.level
       OR NEW.project_id IS DISTINCT FROM OLD.project_id OR NEW.subject_kind IS DISTINCT FROM OLD.subject_kind
       OR NEW.subject_id IS DISTINCT FROM OLD.subject_id OR NEW.capabilities IS DISTINCT FROM OLD.capabilities
       OR NEW.resource_type IS DISTINCT FROM OLD.resource_type OR NEW.resource_value IS DISTINCT FROM OLD.resource_value
       OR NEW.definition_hash IS DISTINCT FROM OLD.definition_hash OR NEW.granted_by IS DISTINCT FROM OLD.granted_by
       OR NEW.created_at IS DISTINCT FROM OLD.created_at OR NEW.expires_at IS DISTINCT FROM OLD.expires_at THEN
        RAISE EXCEPTION 'grant % cannot be edited; revoke it and make a new one', OLD.id;
    END IF;
    IF OLD.status <> 'active' AND NEW.status IS DISTINCT FROM OLD.status THEN
        RAISE EXCEPTION 'grant % is % and cannot change status', OLD.id, OLD.status;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER grants_append_only BEFORE UPDATE OR DELETE ON grants
    FOR EACH ROW EXECUTE FUNCTION grants_are_append_only();
