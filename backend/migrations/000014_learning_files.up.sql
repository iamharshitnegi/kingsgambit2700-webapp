CREATE TABLE teaching_files (
 id uuid PRIMARY KEY, owner_id uuid NOT NULL REFERENCES users(id), student_id uuid REFERENCES student_profiles(id),
 filename text NOT NULL, media_type text NOT NULL, size_bytes bigint NOT NULL CHECK(size_bytes>0),
 sha256 bytea NOT NULL CHECK(octet_length(sha256)=32), object_key text NOT NULL UNIQUE,
 status text NOT NULL CHECK(status IN ('quarantined','clean','rejected','failed')),
 created_at timestamptz NOT NULL DEFAULT now(), scanned_at timestamptz
);
CREATE INDEX teaching_files_owner_idx ON teaching_files(owner_id,created_at DESC);
ALTER TABLE teaching_resources DROP CONSTRAINT teaching_resources_kind_check;
ALTER TABLE teaching_resources ADD CONSTRAINT teaching_resources_kind_check CHECK(kind IN ('pgn','fen','study','pdf','image'));
ALTER TABLE teaching_resources ADD COLUMN file_id uuid REFERENCES teaching_files(id);
ALTER TABLE assignment_evidence ADD COLUMN file_id uuid UNIQUE REFERENCES teaching_files(id);
ALTER TABLE learning_certificates ADD COLUMN file_id uuid REFERENCES teaching_files(id);
