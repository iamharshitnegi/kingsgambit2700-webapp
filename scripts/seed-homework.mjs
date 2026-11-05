// Local Compose fixtures only. No environment-selected database or real accounts.
import { spawnSync } from "node:child_process";
import { sampleHomework } from "../app/lib/demo/homework-samples.ts";

if (process.env.NODE_ENV === "production") throw new Error("Homework fixtures are development-only.");
const samples = sampleHomework("local-student");
const fixtureId = index => `01996000-0000-7000-8000-${String(index + 1).padStart(12, "0")}`;
const quote = value => `'${String(value).replaceAll("'", "''")}'`;
const sql = `
BEGIN;
SELECT pg_advisory_xact_lock(hashtext('kg2700-local-homework-fixtures-v1'));
CREATE TEMP TABLE homework_fixture_accounts AS
SELECT s.id AS student_id, coach.id AS coach_id
FROM student_profiles s JOIN users student ON student.id=s.user_id
JOIN users coach ON coach.email='coach@example.test' AND coach.display_name='Local Coach' AND coach.status='active'
WHERE student.email='student@example.test' AND student.display_name='Local Student' AND student.status='active'
AND EXISTS(SELECT 1 FROM role_grants r WHERE r.user_id=student.id AND r.role='student' AND r.revoked_at IS NULL)
AND EXISTS(SELECT 1 FROM role_grants r WHERE r.user_id=coach.id AND r.role='coach' AND r.revoked_at IS NULL)
AND EXISTS(SELECT 1 FROM enrolments e JOIN batches b ON b.id=e.batch_id WHERE e.student_profile_id=s.id AND e.status='active' AND e.starts_at<=now() AND (e.ends_at IS NULL OR e.ends_at>now()) AND b.coach_user_id=coach.id);
DO $$ BEGIN
  IF (SELECT count(*) FROM homework_fixture_accounts) <> 1 THEN
    RAISE EXCEPTION 'Expected one active Local Student with Local Coach. No fixtures were written.';
  END IF;
END $$;
CREATE TEMP TABLE inserted_homework_fixtures (id uuid);
${samples.map((item, index) => {
  const id = fixtureId(index);
  const content = { puzzles: item.puzzles, resources: item.resources.map(resource => ({ ...resource, id: fixtureId(100 + index) })) };
  return `
WITH inserted AS (
  INSERT INTO learning_assignments(id,assignment_key,student_profile_id,title,instructions,assignment_type,version,status,due_at,authored_by,created_at,content)
  SELECT '${id}','${id}',student_id,${quote(item.title)},${quote(item.description)},${quote(item.kind)},1,${quote(item.status)},${quote(item.dueAt)}::timestamptz,coach_id,now(),${quote(JSON.stringify(content))}::jsonb
  FROM homework_fixture_accounts ON CONFLICT (id) DO NOTHING RETURNING id
) INSERT INTO inserted_homework_fixtures SELECT id FROM inserted;
${item.attempts.map(attempt => `
INSERT INTO assignment_attempts(id,assignment_id,student_profile_id,attempt_number,submitted_payload,status,submitted_at,reviewed_by,reviewed_at,feedback,created_at)
SELECT '${fixtureId(50 + index)}','${id}',student_id,1,${quote(JSON.stringify({ answer: attempt.answer, puzzles: attempt.puzzles }))}::jsonb,${quote(attempt.status)},${attempt.status === "started" ? "NULL" : "now()"},${["reviewed", "returned"].includes(attempt.status) ? "coach_id,now()" : "NULL,NULL"},${quote(attempt.feedback)},now()
FROM homework_fixture_accounts WHERE EXISTS(SELECT 1 FROM inserted_homework_fixtures WHERE id='${id}');`).join("\n")}`;
}).join("\n")}
INSERT INTO audit_events(id,occurred_at,actor_type,action,object_type,object_id,reason_code,reason_text,metadata)
SELECT gen_random_uuid(),now(),'local_fixture','seed_homework','learning_assignment',id::text,'development_sample','Requested local homework design samples','{"source":"scripts/seed-homework.mjs","version":1}'::jsonb
FROM inserted_homework_fixtures;
SELECT count(*) AS new_sample_assignments FROM inserted_homework_fixtures;
COMMIT;
`;
const result = spawnSync("docker", ["compose", "exec", "-T", "postgres", "psql", "-U", "kg2700", "-d", "kg2700", "-X", "-v", "ON_ERROR_STOP=1"], { input: sql, encoding: "utf8", maxBuffer: 1024 * 1024 });
if (result.error) throw result.error;
if (result.status !== 0) { console.error(result.stderr); process.exit(result.status || 1); }
console.log(result.stdout);
console.log("Local homework samples are available at /student/homework and /coach/homework. Existing records were preserved.");
