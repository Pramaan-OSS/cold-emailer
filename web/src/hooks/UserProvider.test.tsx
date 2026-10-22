import React,{useLayoutEffect} from 'react';
import {render,screen} from '@testing-library/react';
import {expect,it,vi} from 'vitest';
const state=vi.hoisted(()=>({rendering:false,clear:vi.fn(),queryClient:{clear:vi.fn()}}));
vi.mock('@/lib/session',()=>({clearClientSession:()=>{expect(state.rendering,'session cleanup must not notify subscribers during render').toBe(false);state.clear();}}));
vi.mock('@tanstack/react-query',()=>({useQueryClient:()=>state.queryClient}));
vi.mock('@/lib/api/hooks/auth/useUser',()=>({default:()=>{state.rendering=true;return{error:{status:401}};}}));
vi.mock('@/lib/api/hooks/app/admin/roles/useRoles',()=>({default:()=>({})}));
vi.mock('@/lib/api/hooks/app/useTimezones',()=>({default:()=>({})}));
vi.mock('@tanstack/react-router',()=>({Navigate:()=> <div>Signed out</div>}));
import {UserProvider} from './UserProvider';
it('cleans up an expired session after rendering and still redirects',()=>{
 function Wrapper(){useLayoutEffect(()=>{state.rendering=false;});return <UserProvider><div>Private workspace</div></UserProvider>;}
 render(<Wrapper/>);expect(screen.getByText('Signed out')).toBeTruthy();expect(screen.queryByText('Private workspace')).toBeNull();expect(state.clear).toHaveBeenCalledTimes(1);
});
