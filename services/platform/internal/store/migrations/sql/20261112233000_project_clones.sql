-- +goose Up
-- Instance-local location, never shared identity or a trust grant (ADR-0066).
CREATE TABLE project_clones (
    project_key text PRIMARY KEY REFERENCES projects(key),
    root text NOT NULL UNIQUE CHECK (root <> '')
);

-- +goose Down
DROP TABLE project_clones;
