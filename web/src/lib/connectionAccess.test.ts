import {describe,it,expect} from 'vitest';
import {connectionAccessNotice} from './connectionAccess';
describe('Connection access notices',()=>{
 it('states full mail access, permanent deletion, profile/contact reads and actual product use',()=>{const notice=connectionAccessNotice('gmail');expect(notice).toContain('permanently delete mail');expect(notice).toContain('profile and contacts');expect(notice).toContain('actions you select or enable');expect(notice).toContain('cannot narrow');});
 it('states full Drive access without promising only chosen-file access',()=>{expect(connectionAccessNotice('googledrive')).toContain('delete every file');});
 it('states read/write and page-picker grants',()=>{expect(connectionAccessNotice('slack')).toContain('read and write');expect(connectionAccessNotice('notion')).toContain('pages you select');});
 it('uses cautious wording for unknown access and unknown apps',()=>{expect(connectionAccessNotice('google_search_console')).toContain('may give this app full access to your Google Search Console');expect(connectionAccessNotice('unknown_app')).toContain('may give this app full access to your connected app account');});
});
