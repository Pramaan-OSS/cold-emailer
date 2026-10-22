-- Preserve upgraded installations’ retired records. Application code never reads these archives.
DO $$
BEGIN
 IF to_regclass('public.pool_link_codes') IS NOT NULL THEN
  ALTER TABLE public.pool_link_codes RENAME TO retired_mail_connection_01;
 END IF;
 IF to_regclass('public.pool_link_instances') IS NOT NULL THEN
  ALTER TABLE public.pool_link_instances RENAME TO retired_mail_connection_02;
 END IF;
 IF to_regclass('public.pool_link_mailboxes') IS NOT NULL THEN
  ALTER TABLE public.pool_link_mailboxes RENAME TO retired_mail_connection_03;
 END IF;
 IF to_regclass('public.cloud_link') IS NOT NULL THEN
  ALTER TABLE public.cloud_link RENAME TO retired_mail_connection_04;
 END IF;
 IF to_regclass('public.cloud_link_mailboxes') IS NOT NULL THEN
  ALTER TABLE public.cloud_link_mailboxes RENAME TO retired_mail_connection_05;
 END IF;
 IF to_regclass('public.cloud_managed_consents') IS NOT NULL THEN
  ALTER TABLE public.cloud_managed_consents RENAME TO retired_mail_connection_06;
 END IF;
 IF to_regclass('public.pool_link_managed_operations') IS NOT NULL THEN
  ALTER TABLE public.pool_link_managed_operations RENAME TO retired_mail_connection_07;
 END IF;
 IF to_regclass('public.mailbox_vendor_connections') IS NOT NULL THEN
  ALTER TABLE public.mailbox_vendor_connections RENAME TO retired_mail_connection_08;
 END IF;
END;
$$;
