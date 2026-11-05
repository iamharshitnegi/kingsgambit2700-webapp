DELETE FROM role_permission_defaults WHERE permission_code LIKE 'learning.%';
DELETE FROM permission_definitions WHERE code LIKE 'learning.%';
DROP TABLE learning_certificates, learning_assessment_history, learning_assessments, learning_rubrics,
 learning_notifications, learning_thread_reads, learning_messages, learning_threads, student_games, teaching_resources;
ALTER TABLE learning_assignments DROP COLUMN content, DROP COLUMN revision;
DROP INDEX learning_assignments_student_created_idx;
-- The published curriculum is intentionally retained: published evidence is immutable.
