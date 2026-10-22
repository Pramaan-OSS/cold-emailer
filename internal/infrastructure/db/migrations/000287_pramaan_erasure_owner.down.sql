DROP TRIGGER IF EXISTS keep_pramaan_erasure_owner ON mailbox_erasures;
DROP FUNCTION IF EXISTS keep_pramaan_erasure_owner();
ALTER TABLE mailbox_erasures DROP COLUMN pramaan_owner;
