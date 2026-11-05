ALTER TABLE classroom_states ADD COLUMN lesson_state jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE classroom_states ADD CONSTRAINT classroom_lesson_object CHECK (jsonb_typeof(lesson_state) = 'object');
