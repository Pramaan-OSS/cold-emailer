-- Upgrade existing local template databases; fresh schemas already use neutral names.
DO $$
DECLARE r record; legacy_prefix text := 'billing'; neutral_prefix text := 'stripe';
BEGIN
 FOR r IN SELECT table_schema, table_name, column_name FROM information_schema.columns WHERE table_schema='public' AND column_name LIKE legacy_prefix || '_%' LOOP
  EXECUTE format('ALTER TABLE %I.%I RENAME COLUMN %I TO %I',r.table_schema,r.table_name,r.column_name,replace(r.column_name,legacy_prefix,neutral_prefix));
 END LOOP;
 FOR r IN SELECT relname FROM pg_class JOIN pg_namespace ON pg_namespace.oid=relnamespace WHERE nspname='public' AND relkind='r' AND relname LIKE legacy_prefix || '_%' LOOP
  EXECUTE format('ALTER TABLE public.%I RENAME TO %I',r.relname,replace(r.relname,legacy_prefix,neutral_prefix));
 END LOOP;
 FOR r IN SELECT c.conname, t.relname FROM pg_constraint c JOIN pg_class t ON t.oid=c.conrelid JOIN pg_namespace n ON n.oid=t.relnamespace WHERE n.nspname='public' AND c.conname LIKE '%' || legacy_prefix || '%' LOOP
  EXECUTE format('ALTER TABLE public.%I RENAME CONSTRAINT %I TO %I',r.relname,r.conname,replace(r.conname,legacy_prefix,neutral_prefix));
 END LOOP;
 FOR r IN SELECT relname FROM pg_class JOIN pg_namespace ON pg_namespace.oid=relnamespace WHERE nspname='public' AND relkind='i' AND relname LIKE '%' || legacy_prefix || '%' LOOP
  EXECUTE format('ALTER INDEX public.%I RENAME TO %I',r.relname,replace(r.relname,legacy_prefix,neutral_prefix));
 END LOOP;
END $$;
