-- Database-enforced immutability for published versions and append-only facts.

CREATE FUNCTION kg_reject_append_only_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME USING ERRCODE = '55000';
END;
$$;

CREATE FUNCTION kg_reject_published_version_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.published_at IS NOT NULL OR
       (to_jsonb(OLD) ? 'status' AND to_jsonb(OLD)->>'status' IN ('published', 'archived')) THEN
        RAISE EXCEPTION 'published row in % is immutable', TG_TABLE_NAME USING ERRCODE = '55000';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER policy_versions_published_immutable
BEFORE UPDATE OR DELETE ON policy_versions
FOR EACH ROW EXECUTE FUNCTION kg_reject_published_version_mutation();

CREATE TRIGGER curriculum_versions_published_immutable
BEFORE UPDATE OR DELETE ON curriculum_versions
FOR EACH ROW EXECUTE FUNCTION kg_reject_published_version_mutation();

CREATE TRIGGER progress_report_versions_published_immutable
BEFORE UPDATE OR DELETE ON progress_report_versions
FOR EACH ROW EXECUTE FUNCTION kg_reject_published_version_mutation();

CREATE TRIGGER consent_records_append_only
BEFORE UPDATE OR DELETE ON consent_records
FOR EACH ROW EXECUTE FUNCTION kg_reject_append_only_mutation();

CREATE TRIGGER entitlement_entries_append_only
BEFORE UPDATE OR DELETE ON entitlement_entries
FOR EACH ROW EXECUTE FUNCTION kg_reject_append_only_mutation();

CREATE TRIGGER payment_transactions_append_only
BEFORE UPDATE OR DELETE ON payment_transactions
FOR EACH ROW EXECUTE FUNCTION kg_reject_append_only_mutation();

CREATE TRIGGER provider_ledger_entries_append_only
BEFORE UPDATE OR DELETE ON provider_ledger_entries
FOR EACH ROW EXECUTE FUNCTION kg_reject_append_only_mutation();

CREATE TRIGGER audit_events_append_only
BEFORE UPDATE OR DELETE ON audit_events
FOR EACH ROW EXECUTE FUNCTION kg_reject_append_only_mutation();

CREATE TRIGGER classroom_events_append_only
BEFORE UPDATE OR DELETE ON classroom_events
FOR EACH ROW EXECUTE FUNCTION kg_reject_append_only_mutation();

CREATE TRIGGER classroom_snapshots_append_only
BEFORE UPDATE OR DELETE ON classroom_snapshots
FOR EACH ROW EXECUTE FUNCTION kg_reject_append_only_mutation();
