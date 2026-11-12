-- +goose Up
-- Instance-local discovery preferences; Project identity and trust remain separate.
CREATE TABLE project_settings (
    project_key text PRIMARY KEY REFERENCES projects(key),
    import_claude_skills boolean NOT NULL DEFAULT false
);
