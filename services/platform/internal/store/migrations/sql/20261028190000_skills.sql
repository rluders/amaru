-- +goose Up
-- Skills (FEAT-SKL-001): procedural knowledge, immutable per scope, Project, name and version. The
-- hash is the SHA-256 of the body, checked by the database so a row cannot hold a hash the Platform
-- did not compute. project_key is empty for an Organization Skill. The trigger refuses an update
-- and a delete: a version a run pinned never changes.
CREATE TABLE skills (
    scope       text        NOT NULL CHECK (scope IN ('organization', 'project')),
    project_key text        NOT NULL DEFAULT '',
    name        text        NOT NULL CHECK (name <> ''),
    version     text        NOT NULL CHECK (version <> '' AND length(version) <= 64),
    body        text        NOT NULL CHECK (body <> '' AND octet_length(body) <= 262144),
    hash        text        NOT NULL CHECK (hash ~ '^[0-9a-f]{64}$'),
    added_by    text        NOT NULL CHECK (added_by <> ''),
    added_at    timestamptz NOT NULL,
    PRIMARY KEY (scope, project_key, name, version),
    CHECK ((scope = 'project') = (project_key <> '')),
    CHECK (hash = encode(sha256(convert_to(body, 'UTF8')), 'hex'))
);

-- +goose StatementBegin
CREATE FUNCTION skills_are_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'skill % % is immutable: it cannot be %', OLD.name, OLD.version, lower(TG_OP);
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER skills_immutable BEFORE UPDATE OR DELETE ON skills
    FOR EACH ROW EXECUTE FUNCTION skills_are_immutable();

-- The Skills a Step pinned, as JSON: [{"name","version","hash"}]. Set when the Step is created.
ALTER TABLE step_executions ADD COLUMN skills jsonb;
