-- Reconcile the original scalar entitlement quantity with the domain's
-- explicit available/reserved/consumed bucket transfers. The append-only
-- trigger is removed only for this transactional migration and restored
-- before the migration completes.

DROP TRIGGER IF EXISTS entitlement_entries_append_only ON entitlement_entries;

ALTER TABLE entitlement_entries
    DROP CONSTRAINT entitlement_entries_entry_type_check,
    DROP CONSTRAINT entitlement_entries_idempotency_key_key,
    ADD COLUMN reference_type text,
    ADD COLUMN reference_id text,
    ADD COLUMN available_delta bigint,
    ADD COLUMN reserved_delta bigint,
    ADD COLUMN consumed_delta bigint,
    ADD COLUMN available_balance bigint,
    ADD COLUMN reserved_balance bigint,
    ADD COLUMN consumed_balance bigint,
    ADD COLUMN request_digest bytea,
    ADD COLUMN recorded_at timestamptz;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM entitlement_entries
        WHERE entry_type <> 'adjust' AND quantity <= 0
    ) THEN
        RAISE EXCEPTION 'legacy entitlement quantities must be positive except for adjustments'
            USING ERRCODE = '23514';
    END IF;
END;
$$;

UPDATE entitlement_entries
SET entry_type = CASE entry_type
        WHEN 'purchase' THEN 'purchased'
        WHEN 'grant' THEN 'granted'
        WHEN 'reserve' THEN 'reserved'
        WHEN 'consume' THEN 'consumed'
        WHEN 'reverse' THEN 'reversed'
        WHEN 'expire' THEN 'expired'
        WHEN 'refund' THEN 'refunded'
        WHEN 'adjust' THEN 'adjusted'
    END,
    reference_type = CASE
        WHEN class_session_id IS NOT NULL THEN 'class_session'
        ELSE 'legacy'
    END,
    reference_id = COALESCE(class_session_id::text, id::text),
    available_delta = CASE entry_type
        WHEN 'purchase' THEN quantity::bigint
        WHEN 'grant' THEN quantity::bigint
        WHEN 'reserve' THEN -quantity::bigint
        WHEN 'consume' THEN 0
        WHEN 'reverse' THEN quantity::bigint
        WHEN 'expire' THEN -quantity::bigint
        WHEN 'refund' THEN -quantity::bigint
        WHEN 'adjust' THEN quantity::bigint
    END,
    reserved_delta = CASE entry_type
        WHEN 'reserve' THEN quantity::bigint
        WHEN 'consume' THEN -quantity::bigint
        ELSE 0
    END,
    consumed_delta = CASE entry_type
        WHEN 'consume' THEN quantity::bigint
        WHEN 'reverse' THEN -quantity::bigint
        ELSE 0
    END,
    request_digest = decode(
        md5(id::text || ':' || idempotency_key) ||
        md5(idempotency_key || ':' || id::text),
        'hex'
    ),
    recorded_at = occurred_at;

WITH running_balances AS (
    SELECT
        id,
        sum(available_delta) OVER ledger_order AS available_balance,
        sum(reserved_delta) OVER ledger_order AS reserved_balance,
        sum(consumed_delta) OVER ledger_order AS consumed_balance
    FROM entitlement_entries
    WINDOW ledger_order AS (
        PARTITION BY account_id
        ORDER BY occurred_at, id
        ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
    )
)
UPDATE entitlement_entries AS entry
SET available_balance = balance.available_balance,
    reserved_balance = balance.reserved_balance,
    consumed_balance = balance.consumed_balance
FROM running_balances AS balance
WHERE balance.id = entry.id;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM entitlement_entries
        WHERE available_balance < 0
           OR reserved_balance < 0
           OR consumed_balance < 0
    ) THEN
        RAISE EXCEPTION 'legacy entitlement history produces a negative bucket balance'
            USING ERRCODE = '23514';
    END IF;
END;
$$;

ALTER TABLE entitlement_entries
    ALTER COLUMN reference_type SET NOT NULL,
    ALTER COLUMN reference_id SET NOT NULL,
    ALTER COLUMN available_delta SET NOT NULL,
    ALTER COLUMN reserved_delta SET NOT NULL,
    ALTER COLUMN consumed_delta SET NOT NULL,
    ALTER COLUMN available_balance SET NOT NULL,
    ALTER COLUMN reserved_balance SET NOT NULL,
    ALTER COLUMN consumed_balance SET NOT NULL,
    ALTER COLUMN request_digest SET NOT NULL,
    ALTER COLUMN recorded_at SET NOT NULL,
    ALTER COLUMN recorded_at SET DEFAULT now(),
    DROP COLUMN quantity,
    ADD CONSTRAINT entitlement_entries_entry_type_check CHECK (
        entry_type IN (
            'purchased', 'granted', 'reserved', 'consumed', 'released',
            'reversed', 'expired', 'refunded', 'adjusted'
        )
    ),
    ADD CONSTRAINT entitlement_entries_reference_type_check CHECK (
        reference_type ~ '^[a-z][a-z0-9_]{0,63}$'
    ),
    ADD CONSTRAINT entitlement_entries_reference_id_check CHECK (
        reference_id <> '' AND length(reference_id) <= 255
    ),
    ADD CONSTRAINT entitlement_entries_reference_link_check CHECK (
        (
            reference_type = 'class_session'
            AND class_session_id IS NOT NULL
            AND reference_id = class_session_id::text
        ) OR (
            reference_type <> 'class_session'
            AND class_session_id IS NULL
        )
    ),
    ADD CONSTRAINT entitlement_entries_reason_check CHECK (
        reason <> '' AND length(reason) <= 500
    ),
    ADD CONSTRAINT entitlement_entries_idempotency_key_check CHECK (
        idempotency_key <> '' AND length(idempotency_key) <= 255
    ),
    ADD CONSTRAINT entitlement_entries_request_digest_check CHECK (
        octet_length(request_digest) = 32
    ),
    ADD CONSTRAINT entitlement_entries_resulting_balance_check CHECK (
        available_balance >= 0 AND reserved_balance >= 0 AND consumed_balance >= 0
    ),
    ADD CONSTRAINT entitlement_entries_delta_shape_check CHECK (
        CASE entry_type
            WHEN 'purchased' THEN
                available_delta > 0 AND reserved_delta = 0 AND consumed_delta = 0
            WHEN 'granted' THEN
                available_delta > 0 AND reserved_delta = 0 AND consumed_delta = 0
            WHEN 'reserved' THEN
                available_delta < 0 AND reserved_delta = -available_delta AND consumed_delta = 0
            WHEN 'consumed' THEN
                (
                    available_delta < 0
                    AND reserved_delta = 0
                    AND consumed_delta = -available_delta
                ) OR (
                    reserved_delta < 0
                    AND available_delta = 0
                    AND consumed_delta = -reserved_delta
                )
            WHEN 'released' THEN
                available_delta > 0 AND reserved_delta = -available_delta AND consumed_delta = 0
            WHEN 'reversed' THEN
                available_delta > 0 AND reserved_delta = 0 AND consumed_delta = -available_delta
            WHEN 'expired' THEN
                available_delta < 0 AND reserved_delta = 0 AND consumed_delta = 0
            WHEN 'refunded' THEN
                available_delta < 0 AND reserved_delta = 0 AND consumed_delta = 0
            WHEN 'adjusted' THEN
                available_delta <> 0 OR reserved_delta <> 0 OR consumed_delta <> 0
            ELSE false
        END
    ),
    ADD CONSTRAINT entitlement_entries_idempotency_account_key
        UNIQUE (account_id, idempotency_key);

DROP INDEX entitlement_entries_account_idx;
CREATE INDEX entitlement_entries_account_idx
    ON entitlement_entries (account_id, occurred_at DESC, id DESC);

CREATE TRIGGER entitlement_entries_append_only
BEFORE UPDATE OR DELETE ON entitlement_entries
FOR EACH ROW EXECUTE FUNCTION kg_reject_append_only_mutation();
