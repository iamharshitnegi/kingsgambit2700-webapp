-- Support workflow hardening. Existing timeline rows default to internal so a
-- rollout cannot accidentally expose historical staff notes to families.

ALTER TABLE support_cases
    ADD COLUMN version bigint NOT NULL DEFAULT 1,
    ADD COLUMN sla_due_at timestamptz;

UPDATE support_cases
SET sla_due_at = opened_at + CASE severity
    WHEN 'critical' THEN interval '1 hour'
    WHEN 'high' THEN interval '4 hours'
    WHEN 'medium' THEN interval '24 hours'
    ELSE interval '72 hours'
END;

UPDATE support_cases
SET resolved_at = COALESCE(resolved_at, updated_at)
WHERE status IN ('resolved', 'closed');

UPDATE support_cases
SET resolved_at = NULL
WHERE status NOT IN ('resolved', 'closed');

ALTER TABLE support_cases
    ALTER COLUMN sla_due_at SET NOT NULL,
    ADD CONSTRAINT support_cases_version_positive CHECK (version > 0),
    ADD CONSTRAINT support_cases_subject_safe CHECK (
        btrim(subject) <> '' AND char_length(subject) <= 200
    ),
    ADD CONSTRAINT support_cases_category_safe CHECK (
        btrim(category) <> '' AND char_length(category) <= 64
    ),
    ADD CONSTRAINT support_cases_sla_after_open CHECK (sla_due_at >= opened_at),
    ADD CONSTRAINT support_cases_resolution_consistent CHECK (
        (status IN ('resolved', 'closed') AND resolved_at IS NOT NULL) OR
        (status NOT IN ('resolved', 'closed') AND resolved_at IS NULL)
    );

ALTER TABLE support_case_events
    ADD COLUMN visibility text NOT NULL DEFAULT 'internal',
    ADD CONSTRAINT support_case_events_visibility_valid CHECK (
        visibility IN ('customer_visible', 'internal')
    ),
    ADD CONSTRAINT support_case_events_type_safe CHECK (
        btrim(event_type) <> '' AND char_length(event_type) <= 64
    ),
    ADD CONSTRAINT support_case_events_body_safe CHECK (
        body IS NULL OR char_length(body) <= 10000
    ),
    ADD CONSTRAINT support_case_events_metadata_object CHECK (
        jsonb_typeof(metadata) = 'object'
    );

CREATE INDEX support_cases_family_cursor_idx
    ON support_cases (family_id, opened_at DESC, id DESC);

CREATE INDEX support_cases_operations_queue_idx
    ON support_cases (status, sla_due_at, assigned_to)
    WHERE status NOT IN ('resolved', 'closed');

CREATE INDEX support_case_events_visible_timeline_idx
    ON support_case_events (support_case_id, occurred_at, id)
    WHERE visibility = 'customer_visible';

CREATE TRIGGER support_case_events_append_only
BEFORE UPDATE OR DELETE ON support_case_events
FOR EACH ROW EXECUTE FUNCTION kg_reject_append_only_mutation();
