import {render,screen,fireEvent,waitFor} from '@testing-library/react';
import {describe,it,expect,vi,beforeEach} from 'vitest';
import ManagedSignIn from './ManagedSignIn';
import {startManagedSignIn} from '@/lib/managedSignIn';
vi.mock('@/lib/managedSignIn',()=>({startManagedSignIn:vi.fn(),managedCallbackURL:()=> 'http://localhost:5173/auth/sso?managed=1'}));
describe('Managed sign-in',()=>{
 beforeEach(()=>vi.resetAllMocks());
 it('does not replace local sign-in when unavailable',()=>{const {container}=render(<ManagedSignIn/>);expect(container).toBeEmptyDOMElement();});
 it('hides account creation when this install refuses signups',()=>{render(<ManagedSignIn authUrl="https://signin.example.test/auth" allowSignup={false}/>);expect(screen.queryByRole('button',{name:'Create an account'})).not.toBeInTheDocument();});
 it('uses the returned backend and the app callback for social sign-in',async()=>{
  const social=vi.fn().mockResolvedValue({data:{}});vi.mocked(startManagedSignIn).mockResolvedValue({signIn:{social}} as never);
  render(<ManagedSignIn authUrl="https://signin.example.test/auth" setupToken="private-claim"/>);
  fireEvent.click(screen.getByRole('button',{name:'Continue with social sign-in'}));
  await waitFor(()=>expect(social).toHaveBeenCalledWith({provider:'google',callbackURL:'http://localhost:5173/auth/sso?managed=1'}));
  expect(startManagedSignIn).toHaveBeenCalledWith('https://signin.example.test/auth','private-claim');
 });
 it('shows a plain refusal without upstream details',async()=>{
  vi.mocked(startManagedSignIn).mockRejectedValue(new Error('private upstream details'));
  render(<ManagedSignIn authUrl="https://signin.example.test/auth"/>);
  fireEvent.click(screen.getByRole('button',{name:'Continue with social sign-in'}));
  expect(await screen.findByRole('alert')).toHaveTextContent('Managed sign-in could not complete.');expect(screen.queryByText(/private upstream/)).not.toBeInTheDocument();
 });
});
