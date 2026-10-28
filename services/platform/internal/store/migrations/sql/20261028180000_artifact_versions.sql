-- +goose Up
-- Artifact versions (FEAT-ART-001, ADR-0017). An Artifact is a stable id; each version records the
-- content hash and size the Platform computed, the media type, where the output was, and the
-- Step attempt that produced it. The content itself lives in the blob store, addressed by hash.
-- Versions are immutable: the trigger refuses an update and a delete. The same attempt recording
-- the same output again is the same version (the unique key), so a retried record adds nothing.
CREATE TABLE artifact_versions (
    artifact_id  text        NOT NULL CHECK (artifact_id <> '' AND length(artifact_id) <= 200),
    version      bigint      NOT NULL CHECK (version >= 1),
    hash         text        NOT NULL CHECK (hash ~ '^sha256:[0-9a-f]{64}$'),
    size         bigint      NOT NULL CHECK (size >= 0),
    media_type   text        NOT NULL DEFAULT '',
    path         text        NOT NULL CHECK (path <> ''),
    execution_id text        NOT NULL CHECK (execution_id <> ''),
    step_id      text        NOT NULL CHECK (step_id <> ''),
    attempt_id   text        NOT NULL CHECK (attempt_id <> ''),
    created_at   timestamptz NOT NULL,
    PRIMARY KEY (artifact_id, version),
    UNIQUE (artifact_id, execution_id, step_id, attempt_id, path)
);
CREATE INDEX artifact_versions_execution ON artifact_versions (execution_id, step_id, artifact_id);

-- +goose StatementBegin
CREATE FUNCTION artifact_versions_are_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'artifact % version % is immutable: it cannot be %', OLD.artifact_id, OLD.version, lower(TG_OP);
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER artifact_versions_immutable BEFORE UPDATE OR DELETE ON artifact_versions
    FOR EACH ROW EXECUTE FUNCTION artifact_versions_are_immutable();
