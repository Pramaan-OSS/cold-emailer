ALTER TABLE ai_mcp_servers ADD COLUMN IF NOT EXISTS connector_ref text NOT NULL DEFAULT '';
ALTER TABLE ai_mcp_servers ADD COLUMN IF NOT EXISTS connector_state text NOT NULL DEFAULT 'failed';
ALTER TABLE ai_mcp_servers ADD COLUMN IF NOT EXISTS read_only boolean NOT NULL DEFAULT true;
ALTER TABLE ai_mcp_servers DROP CONSTRAINT IF EXISTS ai_mcp_servers_auth_type_check;
ALTER TABLE ai_mcp_servers ADD CONSTRAINT ai_mcp_servers_auth_type_check CHECK (auth_type IN ('none','bearer','oauth'));
-- Old disabled direct-server records are not silently enrolled or sent anywhere.
UPDATE ai_mcp_servers SET url='', credentials_encrypted='', enabled=false, discovered_tools='[]', last_error='Reconnect through Pramaan to use this connection.' WHERE connector_ref='';
DROP INDEX IF EXISTS uq_ai_mcp_servers_org_name;
CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_mcp_servers_owner_name ON ai_mcp_servers(org_id, created_by, lower(name));
