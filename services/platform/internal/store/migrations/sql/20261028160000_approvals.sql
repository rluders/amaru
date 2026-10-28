-- +goose Up
-- Approvals (FEAT-REV-001, ADR-0036): a human decision on one gated Step, bound to the Evidence the
-- person was shown. The row is written in the same transaction as the Step change it causes, so a
-- Step in awaiting_approval always has its requested Approval, and a decision and the Step it
-- releases or ends change together. A decided or voided Approval is never edited or deleted; what
-- identifies it (gate, requester, minimum role, evidence) is never edited at all.
CREATE TABLE approvals (
    id              text        PRIMARY KEY,
    version         bigint      NOT NULL CHECK (version >= 1),
    execution_id    text        NOT NULL REFERENCES executions (id),
    step_id         text        NOT NULL REFERENCES step_executions (id),
    project_id      text        NOT NULL CHECK (project_id <> ''),
    state           text        NOT NULL CHECK (state IN ('requested', 'approved', 'rejected', 'expired', 'voided')),
    gate_kind       text        NOT NULL CHECK (gate_kind IN ('policy_required')),
    capability      text        NOT NULL CHECK (capability <> ''),
    requester       jsonb       NOT NULL,
    min_role        text        NOT NULL CHECK (min_role IN ('owner', 'admin', 'maintainer')),
    evidence        jsonb       NOT NULL CHECK (jsonb_typeof(evidence) = 'array' AND jsonb_array_length(evidence) > 0),
    evidence_digest text        NOT NULL CHECK (evidence_digest <> ''),
    decision        jsonb,
    requested_at    timestamptz NOT NULL,
    voided_at       timestamptz,
    void_reason     text        NOT NULL DEFAULT '',
    CONSTRAINT approvals_decided CHECK ((state IN ('approved', 'rejected')) = (decision IS NOT NULL)),
    CONSTRAINT approvals_voided CHECK ((state = 'voided') = (voided_at IS NOT NULL AND void_reason <> ''))
);
CREATE INDEX approvals_by_execution ON approvals (execution_id, requested_at, id);
CREATE INDEX approvals_by_step ON approvals (step_id);

-- +goose StatementBegin
CREATE FUNCTION approvals_are_append_only() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'approval % cannot be deleted', OLD.id;
    END IF;
    IF NEW.id IS DISTINCT FROM OLD.id OR NEW.execution_id IS DISTINCT FROM OLD.execution_id
       OR NEW.step_id IS DISTINCT FROM OLD.step_id OR NEW.project_id IS DISTINCT FROM OLD.project_id
       OR NEW.gate_kind IS DISTINCT FROM OLD.gate_kind OR NEW.capability IS DISTINCT FROM OLD.capability
       OR NEW.requester IS DISTINCT FROM OLD.requester OR NEW.min_role IS DISTINCT FROM OLD.min_role
       OR NEW.evidence IS DISTINCT FROM OLD.evidence OR NEW.evidence_digest IS DISTINCT FROM OLD.evidence_digest
       OR NEW.requested_at IS DISTINCT FROM OLD.requested_at THEN
        RAISE EXCEPTION 'approval % cannot be edited; its gate, requester and evidence are fixed', OLD.id;
    END IF;
    IF OLD.state <> 'requested' THEN
        RAISE EXCEPTION 'approval % is % and cannot change', OLD.id, OLD.state;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER approvals_append_only BEFORE UPDATE OR DELETE ON approvals
    FOR EACH ROW EXECUTE FUNCTION approvals_are_append_only();

-- The audit log now also records what Policy asked for.
ALTER TABLE audit_events DROP CONSTRAINT audit_events_actor_kind_check;
ALTER TABLE audit_events ADD CONSTRAINT audit_events_actor_kind_check CHECK (actor_kind IN ('human', 'platform', 'policy'));
