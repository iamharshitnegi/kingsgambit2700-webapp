CREATE TABLE teaching_resource_versions (
 resource_id uuid NOT NULL REFERENCES teaching_resources(id),
 version integer NOT NULL,
 snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(resource_id,version)
);
INSERT INTO teaching_resource_versions(resource_id,version,snapshot)
 SELECT id,version,to_jsonb(r) FROM teaching_resources r WHERE status='published';

CREATE TRIGGER teaching_resource_versions_append_only
BEFORE UPDATE OR DELETE ON teaching_resource_versions
FOR EACH ROW EXECUTE FUNCTION kg_reject_append_only_mutation();
