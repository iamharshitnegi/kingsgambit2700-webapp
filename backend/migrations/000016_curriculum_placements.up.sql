CREATE TABLE learning_placements (
 id uuid PRIMARY KEY,
 student_id uuid NOT NULL REFERENCES student_profiles(id),
 curriculum_version_id uuid NOT NULL REFERENCES curriculum_versions(id),
 level_slug text NOT NULL,
 previous_level_slug text NOT NULL DEFAULT '',
 report_version_id uuid NOT NULL REFERENCES progress_report_versions(id),
 reason text NOT NULL,
 placed_by uuid NOT NULL REFERENCES users(id),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX learning_placements_student_idx ON learning_placements(student_id,created_at DESC,id DESC);
