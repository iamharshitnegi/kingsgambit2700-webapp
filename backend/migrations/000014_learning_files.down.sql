ALTER TABLE learning_certificates DROP COLUMN file_id;
ALTER TABLE assignment_evidence DROP COLUMN file_id;
ALTER TABLE teaching_resources DROP COLUMN file_id;
ALTER TABLE teaching_resources DROP CONSTRAINT teaching_resources_kind_check;
-- Resource metadata remains readable when rolling back the file pipeline.
ALTER TABLE teaching_resources ADD CONSTRAINT teaching_resources_kind_check CHECK(kind IN ('pgn','fen','study','pdf','image'));
DROP TABLE teaching_files;
