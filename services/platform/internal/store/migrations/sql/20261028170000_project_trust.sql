-- +goose Up
-- Project trust (FEAT-PRJ-003): a human's decision that a Project, or one exact content of it,
-- may be used. One row per Project, kind and ref; a row is replaced when the content is trusted
-- again at a new hash, and a revoked row stays. Every change is also an audit event, written in
-- the same transaction by the store. A trust cannot be deleted.
CREATE TABLE project_trust (
    project_key   text        NOT NULL REFERENCES projects (key),
    kind          text        NOT NULL CHECK (kind IN ('project', 'workflow', 'skill')),
    ref           text        NOT NULL DEFAULT '',
    hash          text        NOT NULL DEFAULT '',
    trusted_by    text        NOT NULL CHECK (trusted_by <> ''),
    trusted_at    timestamptz NOT NULL,
    revoked_by    text        NOT NULL DEFAULT '',
    revoked_at    timestamptz,
    revoke_reason text        NOT NULL DEFAULT '',
    PRIMARY KEY (project_key, kind, ref),
    CHECK ((kind = 'project' AND ref = '' AND hash = '')
        OR (kind <> 'project' AND ref <> '' AND hash ~ '^sha256:[0-9a-f]{64}$')),
    CHECK ((revoked_at IS NULL) = (revoked_by = '')),
    CHECK (revoked_at IS NULL OR revoke_reason <> '')
);

-- +goose StatementBegin
CREATE FUNCTION project_trust_is_not_deleted() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'project trust % % is never deleted: revoke it', OLD.project_key, OLD.kind;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER project_trust_no_delete BEFORE DELETE ON project_trust
    FOR EACH ROW EXECUTE FUNCTION project_trust_is_not_deleted();
