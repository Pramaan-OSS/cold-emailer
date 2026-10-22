-- Preserve old mirror/outbox records without installing or using direct integrations.
-- Fresh installs create none of these tables. Native contacts, CRM records and campaigns stay intact.
DO $$
DECLARE old_names text[] := ARRAY[
 'slack_user_links','slack_link_codes','slack_agent_threads','slack_inbox_threads',
 'salesforce_record_links','salesforce_activity_queue','salesforce_import_sources','salesforce_import_members','salesforce_sync_state',
 'crm_settings','crm_external_links','crm_contact_records','crm_owners','crm_sync_jobs','crm_sync_cursors'];
 idx integer;
 archived text;
BEGIN
 FOR idx IN 1..array_length(old_names,1) LOOP
  archived := 'retired_integration_' || lpad(idx::text,2,'0');
  IF to_regclass(format('public.%I',old_names[idx])) IS NOT NULL AND to_regclass(format('public.%I',archived)) IS NULL THEN
   EXECUTE format('ALTER TABLE public.%I RENAME TO %I',old_names[idx],archived);
  END IF;
 END LOOP;
END $$;
