ALTER TABLE learning_assignments ADD COLUMN content jsonb NOT NULL DEFAULT '{"puzzles":[]}'::jsonb,
    ADD COLUMN revision integer NOT NULL DEFAULT 1;
CREATE INDEX learning_assignments_student_created_idx ON learning_assignments(student_profile_id, created_at DESC);

CREATE TABLE teaching_resources (
    id uuid PRIMARY KEY, owner_id uuid NOT NULL REFERENCES users(id), title text NOT NULL,
    kind text NOT NULL CHECK(kind IN ('pgn','fen','study')),
    visibility text NOT NULL CHECK(visibility IN ('personal','shared')),
    status text NOT NULL CHECK(status IN ('draft','published','archived')),
    level_slug text NOT NULL DEFAULT '', tags text[] NOT NULL DEFAULT '{}',
    content text NOT NULL, version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX teaching_resources_owner_idx ON teaching_resources(owner_id, updated_at DESC);
CREATE TABLE student_games (
    id uuid PRIMARY KEY, student_id uuid NOT NULL REFERENCES student_profiles(id),
    class_id uuid REFERENCES class_sessions(id), owner_id uuid NOT NULL REFERENCES users(id),
    title text NOT NULL, source text NOT NULL CHECK(source IN ('imported','lesson','homework')),
    pgn text NOT NULL, review text NOT NULL DEFAULT '', shared boolean NOT NULL DEFAULT false,
    played_at timestamptz NOT NULL, version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX student_games_student_idx ON student_games(student_id, played_at DESC);
CREATE TABLE learning_threads (
    id uuid PRIMARY KEY, student_id uuid REFERENCES student_profiles(id), batch_id uuid REFERENCES batches(id),
    title text NOT NULL, created_by uuid NOT NULL REFERENCES users(id), created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((student_id IS NULL) <> (batch_id IS NULL))
);
CREATE TABLE learning_messages (
    id uuid PRIMARY KEY, thread_id uuid NOT NULL REFERENCES learning_threads(id),
    actor_id uuid NOT NULL REFERENCES users(id), body text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX learning_messages_thread_idx ON learning_messages(thread_id, created_at, id);
CREATE TABLE learning_thread_reads (
    thread_id uuid NOT NULL REFERENCES learning_threads(id), user_id uuid NOT NULL REFERENCES users(id),
    read_at timestamptz NOT NULL, PRIMARY KEY(thread_id,user_id)
);
CREATE TABLE learning_notifications (
    id uuid PRIMARY KEY, recipient_id uuid NOT NULL REFERENCES users(id), student_id uuid REFERENCES student_profiles(id),
    title text NOT NULL, href text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), read_at timestamptz
);
CREATE INDEX learning_notifications_recipient_idx ON learning_notifications(recipient_id,created_at DESC);
CREATE TABLE learning_rubrics (
    id uuid PRIMARY KEY, level_slug text NOT NULL, version integer NOT NULL,
    title text NOT NULL, dimensions text[] NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(level_slug,version)
);
CREATE TABLE learning_assessments (
    id uuid PRIMARY KEY, student_id uuid NOT NULL REFERENCES student_profiles(id),
    report_id uuid NOT NULL REFERENCES progress_reports(id), report_version_id uuid NOT NULL UNIQUE REFERENCES progress_report_versions(id),
    rubric_id uuid NOT NULL REFERENCES learning_rubrics(id), author_id uuid NOT NULL REFERENCES users(id),
    title text NOT NULL, period text NOT NULL, content jsonb NOT NULL,
    status text NOT NULL CHECK(status IN ('draft','submitted','returned','published')),
    version integer NOT NULL DEFAULT 1, published_at timestamptz, acknowledged_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX learning_assessments_student_idx ON learning_assessments(student_id,created_at DESC);
CREATE TABLE learning_assessment_history (
    id uuid PRIMARY KEY, assessment_id uuid NOT NULL REFERENCES learning_assessments(id),
    actor_id uuid NOT NULL REFERENCES users(id), action text NOT NULL, content jsonb NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE learning_certificates (
    id uuid PRIMARY KEY, student_id uuid NOT NULL REFERENCES student_profiles(id),
    assessment_id uuid NOT NULL REFERENCES learning_assessments(id), level_slug text NOT NULL,
    issuer_id uuid NOT NULL REFERENCES users(id), status text NOT NULL CHECK(status IN ('issued','revoked')),
    issued_at timestamptz NOT NULL DEFAULT now(), revoked_at timestamptz, reason text NOT NULL DEFAULT '',
    UNIQUE(student_id,assessment_id)
);
INSERT INTO permission_definitions(code,description) VALUES
 ('learning.manage','Create assignments and review learner work'),
 ('learning.publish','Publish academy resources and assessments'),
 ('learning.view','Read academy learning records'),
 ('learning.communicate','Participate in guardian-visible learning threads');
INSERT INTO role_permission_defaults(role,permission_code) VALUES
 ('academic_manager','learning.manage'),('academic_manager','learning.publish'),
 ('academic_manager','learning.view'),('academic_manager','learning.communicate'),
 ('operations_admin','learning.communicate'),
 ('super_admin','learning.manage'),('super_admin','learning.publish'),
 ('super_admin','learning.view'),('super_admin','learning.communicate');
INSERT INTO learning_rubrics(id,level_slug,version,title,dimensions) VALUES
 ('01995000-0000-7000-8000-000000000001','foundation-1',1,'Board foundations',ARRAY['Legal play','Piece safety','Checks and threats','Participation','Explanation']),
 ('01995000-0000-7000-8000-000000000002','foundation-2',1,'Tactical foundations',ARRAY['Tactical patterns','Basic mates','Calculation habits','Participation','Game reflection']),
 ('01995000-0000-7000-8000-000000000003','development-1',1,'Calculation and planning',ARRAY['Candidate moves','Calculation','Pawn plans','Endgames','Independent application']),
 ('01995000-0000-7000-8000-000000000004','development-2',1,'Competitive habits',ARRAY['Opening preparation','Clock management','Resilience','Game review','Practice habits']),
 ('01995000-0000-7000-8000-000000000005','performance-1',1,'Advanced performance',ARRAY['Complex calculation','Strategic evaluation','Preparation','Evidence-led review','Independent application']);
INSERT INTO curriculum_versions(id,version,status,effective_at,public_json,created_at,published_at)
VALUES ('01995000-0000-7000-8000-000000000010','academy-framework-v2','published',now(),
 '{"version":"academy-framework-v2","tracks":[{"slug":"foundation","name":"Foundation","summary":"Build sound chess habits.","levels":[{"slug":"foundation-1","name":"Board foundations","summary":"Rules, piece safety and checks.","outcomes":["Play legal moves","Recognise check and simple threats"]},{"slug":"foundation-2","name":"Tactical foundations","summary":"Tactics and elementary mates.","outcomes":["Spot tactical patterns","Review a complete game"]}]},{"slug":"development","name":"Development","summary":"Connect calculation with plans.","levels":[{"slug":"development-1","name":"Calculation and planning","summary":"Candidate moves, plans and endgames.","outcomes":["Calculate forcing variations","Identify plans"]},{"slug":"development-2","name":"Competitive habits","summary":"Preparation, clock management and reflection.","outcomes":["Manage time","Turn game review into practice"]}]},{"slug":"performance","name":"Performance","summary":"Individual competitive development.","levels":[{"slug":"performance-1","name":"Advanced performance","summary":"Complex calculation and tournament preparation.","outcomes":["Compare complex candidate moves","Evaluate progress with evidence"]}]}]}'::jsonb,now(),now());
