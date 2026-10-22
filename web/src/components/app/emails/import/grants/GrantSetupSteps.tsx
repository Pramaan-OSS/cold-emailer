import {Link} from "@tanstack/react-router";
const Unavailable=()=> <div role="status"><p>Directory delegation and tenant discovery are unavailable. Connect individual mailboxes through Pramaan.</p><Link to="/app/integrations">Open Connections</Link></div>;
export const GOOGLE_DELEGATION_URL="/app/integrations";
export function GoogleAuthorizeStep(_props:Record<string,unknown>){return <Unavailable/>;}
export function GoogleVerifyStep(_props:Record<string,unknown>){return <Unavailable/>;}
export function MicrosoftConsentStep(_props:Record<string,unknown>){return <Unavailable/>;}
