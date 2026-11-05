DROP TRIGGER IF EXISTS webhook_receipts_append_only ON webhook_receipts;

ALTER TABLE webhook_receipts
    DROP CONSTRAINT IF EXISTS webhook_receipts_payload_hash_length,
    DROP CONSTRAINT IF EXISTS webhook_receipts_event_id_present,
    DROP CONSTRAINT IF EXISTS webhook_receipts_provider_known;
