-- Keep native records and mail history, but never run a remote pool mirror as an owned mailbox.
DO $$
BEGIN
 IF to_regclass('public.retired_mail_connection_03') IS NOT NULL OR to_regclass('public.retired_mail_connection_05') IS NOT NULL THEN
  CREATE TABLE IF NOT EXISTS public.retired_shared_mailbox_accounts (
   id uuid PRIMARY KEY, payload jsonb NOT NULL, archived_at timestamptz NOT NULL DEFAULT now()
  );
 END IF;
 IF to_regclass('public.retired_mail_connection_03') IS NOT NULL THEN
  INSERT INTO public.retired_shared_mailbox_accounts (id, payload)
   SELECT ea.id, to_jsonb(ea) FROM email_accounts ea JOIN retired_mail_connection_03 r ON r.email_account_id=ea.id
   ON CONFLICT (id) DO NOTHING;
  UPDATE email_accounts SET status='inactive', warmup=NULL, test_mode='off'
   WHERE id IN (SELECT email_account_id FROM retired_mail_connection_03);
 END IF;
 IF to_regclass('public.retired_mail_connection_05') IS NOT NULL THEN
  INSERT INTO public.retired_shared_mailbox_accounts (id, payload)
   SELECT ea.id, to_jsonb(ea) FROM email_accounts ea JOIN retired_mail_connection_05 r ON r.email_account_id=ea.id WHERE r.managed
   ON CONFLICT (id) DO NOTHING;
  UPDATE email_accounts SET status='inactive', warmup=NULL, test_mode='off'
   WHERE id IN (SELECT email_account_id FROM retired_mail_connection_05 WHERE managed);
 END IF;
END;
$$;
