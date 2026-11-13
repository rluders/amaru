-- +goose Up
-- A Project is identified by its key (ADR-0046). The table holds identity and display data only;
-- clone locations, trust and operational state are separate tables keyed by projects.key.
CREATE TABLE projects (
    key        text        PRIMARY KEY,
    name       text        NOT NULL,
    created_at timestamptz NOT NULL
);

-- Instance-local location, never shared identity or a trust grant (ADR-0066).
CREATE TABLE project_clones (
    project_key text PRIMARY KEY REFERENCES projects(key),
    root text NOT NULL UNIQUE CHECK (root <> '')
);
