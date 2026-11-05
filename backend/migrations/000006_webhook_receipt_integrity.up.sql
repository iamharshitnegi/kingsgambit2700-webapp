ALTER TABLE webhook_receipts
    ADD CONSTRAINT webhook_receipts_provider_known
        CHECK (provider IN ('razorpay', 'resend', 'daily')),
    ADD CONSTRAINT webhook_receipts_event_id_present
        CHECK (provider_event_id <> ''),
    ADD CONSTRAINT webhook_receipts_payload_hash_length
        CHECK (octet_length(payload_hash) = 32);

-- A receipt is the immutable evidence that a verified provider delivery was
-- accepted. Downstream projections and River job state live in their own rows;
-- they must never rewrite the signed-delivery evidence.
CREATE TRIGGER webhook_receipts_append_only
BEFORE UPDATE OR DELETE ON webhook_receipts
FOR EACH ROW EXECUTE FUNCTION kg_reject_append_only_mutation();
