CREATE TABLE pramaan_resources (
 owner text NOT NULL,
 kind text NOT NULL,
 handle text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(owner, kind, handle)
);
CREATE UNIQUE INDEX pramaan_private_resource_owner ON pramaan_resources(kind, handle) WHERE kind NOT IN ('action','definition');
