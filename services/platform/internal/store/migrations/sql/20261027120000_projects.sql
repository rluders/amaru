-- +goose Up
-- A Project is identified by its key (ADR-0046). The table holds identity and display data only;
-- clone locations, trust and operational state are separate tables keyed by projects.key.
CREATE TABLE projects (
    key        text        PRIMARY KEY,
    name       text        NOT NULL,
    created_at timestamptz NOT NULL
);
