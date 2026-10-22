-- Retain broker references and the safe auth kinds on rollback; erased secrets are not restored.
DROP INDEX IF EXISTS uq_ai_mcp_servers_owner_name;
