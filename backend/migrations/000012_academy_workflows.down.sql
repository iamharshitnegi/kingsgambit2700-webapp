DELETE FROM role_permission_defaults WHERE permission_code IN ('academy.manage','academy.staff.manage','academy.policy.manage','schedule.manage','schedule.view','teaching.manage');
DELETE FROM permission_grants WHERE permission_code IN ('academy.manage','academy.staff.manage','academy.policy.manage','schedule.manage','schedule.view','teaching.manage');
DELETE FROM permission_definitions WHERE code IN ('academy.manage','academy.staff.manage','academy.policy.manage','schedule.manage','schedule.view','teaching.manage');
DROP TABLE academy_make_up_obligations, academy_request_results, academy_class_participants;
DROP TABLE academy_user_preferences;
ALTER TABLE class_sessions DROP COLUMN recurrence_rule_id, DROP COLUMN meeting_url, DROP COLUMN lesson_notes, DROP COLUMN title;
ALTER TABLE batches DROP COLUMN name;
ALTER TABLE leads DROP COLUMN academy_notes, DROP COLUMN converted_student_id;
ALTER TABLE trial_bookings DROP COLUMN academy_notes;
