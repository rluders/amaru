-- +goose Up
-- The outcome of every mutating command, written in the same transaction as the change it made
-- (FEAT-EXE-005). A request with a stored (name, key) does not run again; it gets result back.
CREATE TABLE commands (
    name        text        NOT NULL,
    key         text        NOT NULL,
    fingerprint text        NOT NULL,
    result      bytea       NOT NULL,
    at          timestamptz NOT NULL,
    PRIMARY KEY (name, key)
);
