ALTER TABLE mailbox_erasures ADD COLUMN pramaan_owner text NOT NULL DEFAULT '';
UPDATE mailbox_erasures SET pramaan_owner=COALESCE(organization_id::text,user_id::text);
CREATE FUNCTION keep_pramaan_erasure_owner() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.pramaan_owner='' THEN NEW.pramaan_owner=COALESCE(NEW.organization_id::text,NEW.user_id::text); END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER keep_pramaan_erasure_owner BEFORE INSERT ON mailbox_erasures FOR EACH ROW EXECUTE FUNCTION keep_pramaan_erasure_owner();
