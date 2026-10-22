-- Local redirect metadata only.
ALTER TABLE domain_redirects ADD COLUMN IF NOT EXISTS cloud_link_instance_id uuid;
