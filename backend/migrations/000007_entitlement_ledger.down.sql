-- Downgrade only when every persisted entry can be represented by the former
-- scalar quantity. Refusing a lossy rollback is safer than silently changing
-- financial meaning.

DROP TRIGGER IF EXISTS entitlement_entries_append_only ON entitlement_entries;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM entitlement_entries WHERE entry_type = 'released') THEN
        RAISE EXCEPTION 'cannot downgrade entitlement ledger containing released entries'
            USING ERRCODE = '23514';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM entitlement_entries
        WHERE entry_type = 'consumed' AND available_delta <> 0
    ) THEN
        RAISE EXCEPTION 'cannot downgrade consumption directly from the available bucket'
            USING ERRCODE = '23514';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM entitlement_entries
        WHERE entry_type = 'adjusted'
          AND (reserved_delta <> 0 OR consumed_delta <> 0)
    ) THEN
        RAISE EXCEPTION 'cannot downgrade multi-bucket entitlement adjustments'
            USING ERRCODE = '23514';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM entitlement_entries
        GROUP BY idempotency_key
        HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'cannot restore globally unique entitlement idempotency keys'
            USING ERRCODE = '23505';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM entitlement_entries
        WHERE greatest(
            abs(available_delta), abs(reserved_delta), abs(consumed_delta)
        ) > 2147483647
    ) THEN
        RAISE EXCEPTION 'cannot fit entitlement quantity into the legacy integer column'
            USING ERRCODE = '22003';
    END IF;
END;
$$;

ALTER TABLE entitlement_entries
    DROP CONSTRAINT entitlement_entries_idempotency_account_key,
    DROP CONSTRAINT entitlement_entries_delta_shape_check,
    DROP CONSTRAINT entitlement_entries_resulting_balance_check,
    DROP CONSTRAINT entitlement_entries_request_digest_check,
    DROP CONSTRAINT entitlement_entries_idempotency_key_check,
    DROP CONSTRAINT entitlement_entries_reason_check,
    DROP CONSTRAINT entitlement_entries_reference_link_check,
    DROP CONSTRAINT entitlement_entries_reference_id_check,
    DROP CONSTRAINT entitlement_entries_reference_type_check,
    DROP CONSTRAINT entitlement_entries_entry_type_check,
    ADD COLUMN quantity integer;

UPDATE entitlement_entries
SET quantity = CASE entry_type
        WHEN 'purchased' THEN available_delta::integer
        WHEN 'granted' THEN available_delta::integer
        WHEN 'reserved' THEN reserved_delta::integer
        WHEN 'consumed' THEN consumed_delta::integer
        WHEN 'reversed' THEN available_delta::integer
        WHEN 'expired' THEN (-available_delta)::integer
        WHEN 'refunded' THEN (-available_delta)::integer
        WHEN 'adjusted' THEN available_delta::integer
    END,
    entry_type = CASE entry_type
        WHEN 'purchased' THEN 'purchase'
        WHEN 'granted' THEN 'grant'
        WHEN 'reserved' THEN 'reserve'
        WHEN 'consumed' THEN 'consume'
        WHEN 'reversed' THEN 'reverse'
        WHEN 'expired' THEN 'expire'
        WHEN 'refunded' THEN 'refund'
        WHEN 'adjusted' THEN 'adjust'
    END;

ALTER TABLE entitlement_entries
    ALTER COLUMN quantity SET NOT NULL,
    ADD CONSTRAINT entitlement_entries_quantity_check CHECK (quantity <> 0),
    DROP COLUMN reference_type,
    DROP COLUMN reference_id,
    DROP COLUMN available_delta,
    DROP COLUMN reserved_delta,
    DROP COLUMN consumed_delta,
    DROP COLUMN available_balance,
    DROP COLUMN reserved_balance,
    DROP COLUMN consumed_balance,
    DROP COLUMN request_digest,
    DROP COLUMN recorded_at,
    ADD CONSTRAINT entitlement_entries_entry_type_check CHECK (
        entry_type IN ('purchase', 'grant', 'reserve', 'consume', 'reverse', 'expire', 'refund', 'adjust')
    ),
    ADD CONSTRAINT entitlement_entries_idempotency_key_key UNIQUE (idempotency_key);

DROP INDEX entitlement_entries_account_idx;
CREATE INDEX entitlement_entries_account_idx
    ON entitlement_entries (account_id, occurred_at);

CREATE TRIGGER entitlement_entries_append_only
BEFORE UPDATE OR DELETE ON entitlement_entries
FOR EACH ROW EXECUTE FUNCTION kg_reject_append_only_mutation();
