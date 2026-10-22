import {describe,expect,it} from 'vitest';
import {MAIL_HOST_LABELS,mailHostLabel,mailboxConnectionLabel,mailHostFamily} from './mailHost';
import {mailboxBrand,mailboxSource} from './mailboxSource';
describe('neutral mailbox artwork and labels',()=>{
 it('keeps all known and unknown host identifiers out of displayed labels',()=>{for(const host of [...Object.keys(MAIL_HOST_LABELS),'future-private-host']){expect(mailHostLabel(host)).toBe('Managed mailbox');expect(mailboxConnectionLabel({mail_host:host,auth_method:'app_password'})).toBe('Managed mailbox · App password');}expect(mailboxConnectionLabel({provider:'future-private-host'})).toBe('Managed mailbox');});
 it('uses one neutral mark even for classified and delegated mailboxes',()=>{for(const host of ['google_workspace','microsoft365','godaddy','namecheap','future-private-host']){const box={provider:'smtp_imap',mail_host:host,auth_method:'delegated' as const,vendor:''};expect(mailboxBrand(box)).toBe('mailbox');expect(mailboxSource(box)).toMatchObject({logo:'mailbox',label:'Admin grant',title:'Connected through an administrator grant'});}});
 it('preserves internal delivery-family matching separately from artwork',()=>{expect(mailHostFamily('google_workspace')).toBe('google');expect(mailHostFamily('microsoft365')).toBe('microsoft');expect(mailHostFamily('godaddy')).toBeNull();});
});
