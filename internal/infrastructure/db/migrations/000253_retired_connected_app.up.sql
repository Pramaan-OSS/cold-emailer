-- Retired direct integration schema. Connected actions use broker-owned records.
SELECT 1;
-- Native CRM rules may hold local campaign leads; no external mirror is needed.
ALTER TABLE public.campaign_leads DROP CONSTRAINT IF EXISTS campaign_leads_pause_source_check;
ALTER TABLE public.campaign_leads ADD CONSTRAINT campaign_leads_pause_source_check
CHECK (pause_source IS NULL OR pause_source IN ('manual', 'out_of_office', 'inbox_tagging', 'cc', 'crm')) NOT VALID;
