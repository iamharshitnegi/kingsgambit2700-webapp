DROP TRIGGER IF EXISTS support_case_events_append_only ON support_case_events;

DROP INDEX IF EXISTS support_case_events_visible_timeline_idx;
DROP INDEX IF EXISTS support_cases_operations_queue_idx;
DROP INDEX IF EXISTS support_cases_family_cursor_idx;

ALTER TABLE support_case_events
    DROP CONSTRAINT IF EXISTS support_case_events_metadata_object,
    DROP CONSTRAINT IF EXISTS support_case_events_body_safe,
    DROP CONSTRAINT IF EXISTS support_case_events_type_safe,
    DROP CONSTRAINT IF EXISTS support_case_events_visibility_valid,
    DROP COLUMN IF EXISTS visibility;

ALTER TABLE support_cases
    DROP CONSTRAINT IF EXISTS support_cases_resolution_consistent,
    DROP CONSTRAINT IF EXISTS support_cases_sla_after_open,
    DROP CONSTRAINT IF EXISTS support_cases_category_safe,
    DROP CONSTRAINT IF EXISTS support_cases_subject_safe,
    DROP CONSTRAINT IF EXISTS support_cases_version_positive,
    DROP COLUMN IF EXISTS sla_due_at,
    DROP COLUMN IF EXISTS version;
