import {render,screen,cleanup} from '@testing-library/react';
import {afterEach,describe,it,expect,vi} from 'vitest';
import PramaanSetupNotice from './PramaanSetupNotice';
const state=vi.hoisted(()=>({config:{pramaan_configured:false as boolean|undefined}}));
vi.mock('@/lib/api/hooks/auth/useAuthConfig',()=>({default:()=>state}));
afterEach(cleanup);
describe('local startup notice',()=>{
 it('shows the one action when there is no key',()=>{state.config.pramaan_configured=false;render(<PramaanSetupNotice/>);expect(screen.getByRole('status').textContent).toBe('Add PRAMAAN_API_KEY to .env');});
 it('does not claim a key is missing while settings are loading or configured',()=>{for(const value of [true,undefined]){state.config.pramaan_configured=value;const {container}=render(<PramaanSetupNotice/>);expect(container.textContent).toBe('');cleanup();}});
});
