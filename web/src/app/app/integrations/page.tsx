import CompanyMailConnect from '@/components/app/emails/CompanyMailConnect';
import MailboxTools from '@/components/app/emails/MailboxTools';
import useAuthConfig from '@/lib/api/hooks/auth/useAuthConfig';
import DomainPurchase from "@/components/app/emails/domains/DomainPurchase";
import listIntegrationConnections from '@/lib/api/client/app/integrations/listConnections';
import updateConnectionConfig from '@/lib/api/client/app/integrations/updateConnectionConfig';
import { useEffect, useRef, useState } from 'react';
import { capability, outcomes, plainLabel, connectedResultRows, connectedDisplayURL, OutcomeError, cataloguePage, connectedFiles, parseConnectedFile, type CatalogueInput } from '@/lib/pramaan';
import { prepareLeadSearch, runLeadSearch, type LeadPlan } from '@/lib/leadOutcome';
import { prepareMailbox, runMailbox } from '@/lib/mailboxOutcome';
import { connectionAccessNotice, redirectToConnection } from '@/lib/connectionAccess';
import { useConfirm } from '@/hooks/context/confirm';
import type { OutcomeBasis } from '@pramaan/sdk';

type Row = Record<string, unknown>;
type Schema = { type?: string; title?: string; description?: string; enum?: unknown[]; properties?: Record<string, Schema>; required?: string[] };
const rowList = connectedResultRows;
const detailLabels: Record<string,string> = {name:'Name',fullName:'Full name',firstName:'First name',lastName:'Last name',jobTitle:'Job title',companyName:'Company',industry:'Industry',description:'Description',domain:'Domain',city:'City',country:'Country',phone:'Phone',employeeCount:'Employees',email:'Email'};
function detailRows(value:Row):[string,string][] {const found:[string,string][]=[];for(const [key,item] of Object.entries(value)){if(detailLabels[key] && (typeof item==='string'||typeof item==='number'))found.push([detailLabels[key],String(item).slice(0,1000)]);else if(['person','company','profile','data'].includes(key) && item && typeof item==='object'&&!Array.isArray(item))found.push(...detailRows(item as Row));}return found;}
const Pin = ({ value }: { value: string }) => <span className="pin" data-pin={value}><i />{value}</span>;

export default function IntegrationsPage() {
  const {config: deployment} = useAuthConfig();
  const [view, setView] = useState(window.location.hash === '#owned-domain' ? 'Your domain' : 'Apps');
  const [query, setQuery] = useState(() => (new URLSearchParams(window.location.search).get('query') ?? '').slice(0, 200));
  const [appFilter, setAppFilter] = useState('');
  const [catalogueInput, setCatalogueInput] = useState<CatalogueInput>();
  const [nextCursor, setNextCursor] = useState<string>();
  const [fileText, setFileText] = useState<string>();
  const [rows, setRows] = useState<Row[]>([]);
  const [selected, setSelected] = useState<Row>();
  const [schema, setSchema] = useState<Schema>();
  const [values, setValues] = useState<Record<string, unknown>>({});
  const [status, setStatus] = useState('Needs you');
  const [message, setMessage] = useState('Choose what you want to connect.');
  const [result, setResult] = useState<Row>();
  const [domain, setDomain] = useState('');
  const [mailboxName, setMailboxName] = useState('');
  const [username, setUsername] = useState('');
  const [leadEmail, setLeadEmail] = useState('');
  const [leadDomain, setLeadDomain] = useState('');
  const [leadRoles, setLeadRoles] = useState('');
  const [leadBasis, setLeadBasis] = useState('');
  const confirm = useConfirm();
  const inputRevision = useRef(0);
  const leadRunning = useRef(false);
  const [leadJob, setLeadJob] = useState<string>();
  const [cancelSent, setCancelSent] = useState(false);
  const cancelLock = useRef(false);
  const requestKey = useRef(crypto.randomUUID());
  const busy = status === 'Working';
  const mailboxRunning = useRef(false);
  const [mailboxSubmitted, setMailboxSubmitted] = useState(false);

  async function connectMailbox(check = false) {
    if (mailboxRunning.current) return;
    mailboxRunning.current = true;
    setStatus('Working'); setResult(undefined); setMessage('Preparing your mailbox connection estimate.');
    try {
      const plan = await prepareMailbox(check);
      const { estimate } = plan;
      const maximum = (estimate.priceMaxMicro / 1_000_000).toFixed(6);
      const description = `${check ? 'Check your existing connection' : 'Connect your own mailbox'}. Steps: ${estimate.steps.filter(step => step.status === 'will_run').map(step => plainLabel(step.label)).join('; ')}. The read test reads at most one message; no message content is returned. Nothing is sent. Successful sign-in starts the mailbox monthly rental. This estimate expires in 15 minutes.`;
      const approved = await new Promise<boolean>(resolve => confirm.show(<>{estimate.priceTypicalMicro !== undefined ? <><strong>usually about ${(estimate.priceTypicalMicro / 1_000_000).toFixed(6)}</strong><small className="block">at most ${maximum}</small></> : <span>Maximum total: ${maximum}. </span>}<span>{description}</span></>, () => resolve(true), () => resolve(false)));
      if (!approved) { setStatus('Needs you'); setMessage('Mailbox connection was cancelled; nothing ran.'); return; }
      if (!check) setMailboxSubmitted(true);
      const { result: connection, complete } = await runMailbox(plan);
      setResult(connection);
      if (typeof connection.connectUrl === 'string') {
        setStatus('Needs you'); setMessage('Finish sign-in, then check your connection.');
        redirectToConnection(connection.connectUrl); return;
      }
      setStatus(complete && connection.tested === true ? 'Done' : 'Needs you');
      setMessage(connection.tested === true ? 'Mailbox read access was tested. Nothing was sent.' : connection.status === 'connected' ? 'Mailbox connected; read access is still unconfirmed. Check your connection again later.' : 'Mailbox sign-in is still pending or failed. Check your connection after signing in.');
    } catch (error) { if (error instanceof OutcomeError && error.effect === 'none') setMailboxSubmitted(false); setStatus('Blocked'); setMessage(error instanceof Error ? error.message : 'Check your connection before trying again.'); }
    finally { mailboxRunning.current = false; }
  }

  async function run(id: string, input: Row, done: (out: Row) => void = setResult) {
    setStatus('Working'); setResult(undefined); setMessage('Pramaan is completing your request.');
    try { const out = await capability<Row>(id, input, requestKey.current); done(out);
      if (id === "apps.action.execute" && out.status !== "succeeded") throw new Error("The action failed. Check its result before retrying; it was not repeated."); setFileText(undefined); setStatus('Done'); setMessage('Your request is complete.'); requestKey.current = crypto.randomUUID(); }
    catch (e) { setStatus('Blocked'); setMessage(e instanceof Error ? e.message : 'The request could not complete.'); }
  }
  function changed(update: () => void) { inputRevision.current++; requestKey.current = crypto.randomUUID(); update(); }
  useEffect(() => { inputRevision.current++; setRows([]); setCatalogueInput(undefined); setNextCursor(undefined); setFileText(undefined); setSelected(undefined); setSchema(undefined); setResult(undefined); setMessage('Choose what you want to connect.'); setStatus('Needs you'); }, [view]);

  async function discover(app?: string, more = false) {
    const requestedApp = app ?? appFilter.trim();
    const input = more ? catalogueInput : { query: query.trim(), ...(requestedApp ? { app: requestedApp } : {}), limit: 50 };
    if (!input || (more && !nextCursor)) return;
    if (!more) { setRows([]); setSelected(undefined); setSchema(undefined); setNextCursor(undefined); }
    setStatus('Working'); setMessage('Loading the requested catalogue page.');
    try {
      const out = await cataloguePage({ ...input, ...(more ? { cursor: nextCursor } : {}) });
      setCatalogueInput(input); setNextCursor(out.nextCursor); setRows(previous => more ? [...previous, ...out.results.filter(row => !previous.some(old => old.action === row.action && old.app === row.app))] : out.results);
      setStatus('Done'); setMessage('Catalogue page ready. No more pages were loaded.');
    } catch (e) { setStatus('Blocked'); setMessage(e instanceof Error ? e.message : 'Catalogue search failed.'); }
  }
  async function readFile(id: string) {
    setStatus('Working'); setMessage('Reading the saved file.');
    try { const parsed = await parseConnectedFile(id); setFileText(parsed.text); setStatus('Done'); setMessage(parsed.truncated ? 'The text is incomplete; the action result is retained.' : 'The saved file was read; the action result is retained.'); }
    catch { setStatus('Blocked'); setMessage('The file could not be read. The action result is retained; do not execute the action again.'); }
  }
  async function searchLeads() {
    if (leadRunning.current) return;
    leadRunning.current = true;
    setLeadJob(undefined); setCancelSent(false); cancelLock.current = false; setResult(undefined);
    const revision = inputRevision.current;
    setStatus('Working'); setMessage('Preparing your lead-search estimate.');
    try {
      let plan: LeadPlan = await prepareLeadSearch({ description: query.trim(), roles: leadRoles.split(',').map(r => r.trim()).filter(Boolean), basis: { ground: leadBasis as OutcomeBasis['ground'] } });
      while (true) {
        const price = (plan.priceMaxMicro / 1_000_000).toFixed(6);
        const text = `Search: ${plan.input.description}. Roles: ${plan.input.roles.join(', ')}. Basis: ${plan.input.basis.ground}. Steps: ${plan.steps.map(plainLabel).join('; ')}. Maximum total: $${price}. This estimate expires in 15 minutes. ${plan.source === 'capability' ? 'The outcome is unavailable; this runs company search only.' : 'Nothing is sent or saved to a CRM.'}`;
        const approved = await new Promise<boolean>(resolve => confirm.show(plan.priceTypicalMicro !== undefined ? <><strong>usually about ${(plan.priceTypicalMicro / 1_000_000).toFixed(6)}</strong><small className="block">at most ${price}</small><span>{text.replace(`Maximum total: $${price}. `, "")}</span></> : text, () => { resolve(true); }, () => resolve(false)));
        if (!approved) { setStatus('Needs you'); setMessage('Lead search was cancelled; nothing ran.'); return; }
        if (revision !== inputRevision.current) throw new Error('The input changed. Review a new estimate.');
        const out = await runLeadSearch(plan, undefined, setLeadJob);
        if (out.fallback) { plan = out.fallback; continue; }
        setResult(out.result); setStatus('Done'); setMessage(plan.source === 'capability' ? 'Company search is complete. Contact matching was unavailable.' : 'Lead search is complete. Missing details remain unknown.'); return;
      }
    } catch (e) { setStatus(e instanceof OutcomeError && e.code === 'rate_limited' ? 'Needs you' : 'Blocked'); setMessage(e instanceof Error ? e.message : 'The run is uncertain. Check before trying again.'); }
    finally { leadRunning.current = false; setLeadJob(undefined); }
  }

  async function cancelLeadRun() {
    if (!leadJob || cancelLock.current) return;
    cancelLock.current = true; setCancelSent(true);
    try { await outcomes.cancel(leadJob); setMessage('Cancellation requested. Waiting for the recorded result; check any unknown steps before retrying.'); }
    catch { setMessage('Cancellation could not be confirmed. Check this run before retrying; the request will not be sent again automatically.'); }
  }

  async function select(row: Row) {
    setSelected(row); setValues({}); setSchema(undefined);
    const candidate = row.inputSchema as Schema | undefined;
    if (candidate && typeof candidate === 'object' && candidate.type === 'object' && candidate.properties) setSchema(candidate);
    else setMessage('This catalogue result has no input schema. Refine your keyword search.');
  }
  async function saveAction() {
    if (!selected || !schema) return;
    setStatus('Working');
    try {
      const {connections} = await listIntegrationConnections();
      const connection = connections.find(row => row.provider === selected.app && row.status === 'connected');
      if (!connection) { setStatus('Needs you'); setMessage('Connect this account, then save the selected action.'); return; }
      await updateConnectionConfig({connectionId:connection.id, config_capabilities:{...connection.config_capabilities,pramaan_action:selected.action,pramaan_input:values},sync_direction:'push'});
      setStatus('Done'); setMessage('Saved for contact actions. Use templates such as {{.email}} for contact fields.');
    } catch { setStatus('Needs you'); setMessage('The action settings could not be saved. Check your connection and permissions.'); }
  }
  const missingFields = schema?.required?.some(name => values[name] === undefined || values[name] === '') ?? false;
  return <section className="capabilities-view" aria-label="Connections">
    <h1>Connections</h1>
    <p>Connected apps are not set up yet until you choose an app and connect your account. Custom sending domains are not set up yet until you attach a domain below.</p>
    {deployment.connector_oauth_configured===false && <p>Remote connector sign-in is not set up yet. Set PRAMAAN_CONNECTOR_RETURN_URL to a public HTTPS return address. Local accounts work without it.</p>}
    <p>Connect your work through Pramaan.</p>
    <nav className="connection-tabs" role="tablist" aria-label="Connection views">{['Apps', 'Mailboxes', 'Your domain', 'Lead data', 'Incoming events'].map(name => <button type="button" className="btn" aria-pressed={view === name} key={name} onClick={() => setView(name)}>{name}</button>)}</nav>
    <div className="state" role="status" aria-live="polite"><Pin value={status} /><span>{message}</span></div>
    {leadJob && <button className="btn" type="button" disabled={cancelSent} onClick={cancelLeadRun}>{cancelSent ? 'Cancellation requested' : 'Cancel lead search'}</button>}
    {view === 'Apps' && <>
      <div className="card"><h2>Your connected apps</h2><button className="btn" disabled={busy} onClick={() => void run('apps.connections.list', {}, out => { setNextCursor(undefined); setCatalogueInput(undefined); setRows(rowList(out)); })}>Show connected apps</button></div>
      <form className="card" onSubmit={e => { e.preventDefault(); void discover(); }}>
        <h2>Find an action</h2><p>Search by keyword. Each new page costs USD 0.001. Pages are cached for this session; more pages load only when you ask.</p><label className="field">What do you want to do?<input className="in" value={query} onChange={e => changed(() => setQuery(e.target.value))} placeholder="Add a contact or update a spreadsheet" /></label>
        <label className="field">App filter (optional)<input className="in" value={appFilter} onChange={e => changed(() => setAppFilter(e.target.value))} placeholder="App name from a catalogue result" /></label><button className="btn" disabled={busy || !query.trim()}>Find actions</button>{!query.trim() && <small>Describe the action first.</small>}
      </form>
      {rows.length > 0 && <div className="card"><h2>Available actions</h2>{rows.map((row, index) => <div className="tile" key={String(row.action ?? row.app ?? index)}>{typeof row.app === 'string' && <p>App: {plainLabel(row.app)}</p>}<p>{plainLabel(row.title ?? row.description ?? row.app) || `Connected action ${index + 1}`} {typeof row.status === 'string' && plainLabel(row.status)}</p>{typeof row.action === 'string' ? <button className="btn btn-sm" disabled={busy} onClick={() => void select(row)}>Choose action</button> : <button className="btn btn-sm" disabled={busy} onClick={() => { if(window.confirm('Disconnect this app and revoke its access?')) void run('apps.connection.revoke', {app:row.app}, () => setRows(rows.filter(item => item.app !== row.app))); }}>Disconnect app</button>}</div>)}</div>}
      {nextCursor && <button className="btn" disabled={busy} onClick={() => void discover(undefined, true)}>Load more actions (USD 0.001 per new page)</button>}
      {selected && <div className="card"><h2>Connect and run</h2><p>App: {plainLabel(selected.app)}</p><p>For bulk contact actions, use field templates such as {'{{.email}}'} and {'{{.first_name}}'}, then save these settings.</p><p>{plainLabel(selected.description) || 'Connect your account before running this action.'}</p><p data-testid="connection-access-notice">{connectionAccessNotice(selected.app)}</p><p>The lab has verified mailbox label and message reads. Other actions, including creating, changing, sending or deleting data, are not yet live-verified and require confirmation.</p>
        <button className="btn" disabled={busy} onClick={() => void run('apps.connection.start', { app: selected.app }, out => { redirectToConnection(out.connectUrl); })}>Connect your account</button>
        <button className="btn" disabled={busy} onClick={() => void discover(String(selected.app))}>Search this app</button>
        {schema?.properties && Object.entries(schema.properties).map(([name, field]) => <label className="field" key={name}>{plainLabel(field.title ?? name.replaceAll('_', ' '))}
          {field.enum ? <select className="in" value={String(values[name] ?? '')} onChange={e => changed(() => setValues({ ...values, [name]: e.target.value }))}><option value="">Choose a value</option>{field.enum.map(value => <option key={String(value)} value={String(value)}>{plainLabel(String(value))}</option>)}</select>
            : field.type === 'boolean' ? <input type="checkbox" checked={values[name] === true} onChange={e => changed(() => setValues({ ...values, [name]: e.target.checked }))} />
              : <input className="in" type={field.type === 'number' || field.type === 'integer' ? 'number' : 'text'} value={String(values[name] ?? '')} onChange={e => changed(() => setValues({ ...values, [name]: field.type === 'number' || field.type === 'integer' ? Number(e.target.value) : e.target.value }))} />}
        </label>)}
        <button className="btn" disabled={busy || !schema || missingFields} onClick={() => void saveAction()}>Save for contact actions</button>
        <button className="btn" disabled={busy || !schema || missingFields} onClick={() => { const action = selected.action; const fields = structuredClone(values); confirm.show(`Run this catalogue action with these fields? ${JSON.stringify(fields)}. Actions that create, change, send or delete data are not yet live-verified.`, () => run('apps.action.execute', { action, input: fields })); }}>Run this action</button>{(!schema || missingFields) && <small>{!schema ? 'Field coverage needs verification.' : 'Complete the required fields first.'}</small>}
      </div>}
    </>}
    {view === 'Mailboxes' && <>
      <form className="card" onSubmit={e => { e.preventDefault(); void run('email.inbox.create', { username, displayName: mailboxName }); }}><h2>Create a mailbox</h2>
        <label className="field">Mailbox name<input className="in" value={mailboxName} onChange={e => changed(() => setMailboxName(e.target.value))} /></label>
        <label className="field">Address name<input className="in" value={username} onChange={e => changed(() => setUsername(e.target.value))} placeholder="hello" /></label>
        <p>Pramaan supplies the domain for this mailbox.</p><button className="btn" disabled={busy || !username.trim()}>Create mailbox</button>{!username.trim() && <small>Enter an address name first.</small>}
      </form>
      <CompanyMailConnect/><MailboxTools/><div className="card"><h2>Use an existing mailbox</h2><p>Connect a mailbox you already own.</p><p data-testid="mailbox-access-notice">{connectionAccessNotice(undefined,'mailbox')}</p><button className="btn" disabled={busy || mailboxSubmitted} onClick={() => void connectMailbox()}>Connect your mailbox</button>
        <button className="btn" disabled={busy} onClick={() => void connectMailbox(true)}>Check your connection</button>
        <button className="btn" disabled={busy} onClick={() => void run('email.inboxes.list', { limit: 50 }, out => setRows(rowList(out)))}>Show your mailboxes</button>
        {rows.map((row, i) => <div className="tile" key={i}>{String(row.address ?? '')}</div>)}
      </div>
    </>}
    {view === 'Your domain' && <>
      <DomainPurchase />
      <form className="card" onSubmit={e => { e.preventDefault(); void run('email.domain.create', { domain: domain.trim(), inbound: false }); }}><h2>Bring a domain you own</h2><p>Your current incoming mail stays with its receiver.</p><label className="field">Your domain<input className="in" value={domain} onChange={e => changed(() => setDomain(e.target.value))} placeholder="example.com" /></label><button className="btn" disabled={busy || !domain.trim()}>Attach your domain</button>{!domain.trim() && <small>Enter your domain first.</small>}</form>
      <div className="card"><h2>Domain ideas</h2><p>Check suggestions for a company name. This does not buy a domain or verify ownership.</p><label className="field">Company name<input className="in" value={query} onChange={e => changed(() => setQuery(e.target.value))} /></label><button className="btn" disabled={busy || !query.trim()} onClick={() => void run('web.domain-names.check', { companyName: query.trim() })}>Check domain suggestions</button></div>
      <div className="card"><h2>Set up your records</h2><p>Copy the records to your domain settings, then check them.</p>{['email.domain.get', 'email.domain.zonefile.get', 'email.domain.setuplink.get', 'email.domain.verify'].map((id, index) => <button className="btn" key={id} disabled={busy || !domain.trim()} onClick={() => void run(id, { domain: domain.trim() })}>{['Show DNS records', 'Read zone file', 'Get setup link', 'Check your records'][index]}</button>)}</div>
      <div className="card"><h2>App web domain</h2><p>Web-domain hosting is not set up yet until the owner sets PRAMAAN_HOSTING_PROJECT. Mail domains do not need that setting.</p>{['domains.add','domains.verify','domains.list'].map((id,i)=><button className="btn" key={id} disabled={busy || deployment.hosting_configured===false || (i<2&&!domain.trim())} onClick={()=>void run(id,i===2?{limit:20}:{domain:domain.trim()})}>{['Attach web domain','Verify web domain','Show web domains'][i]}</button>)}</div>
    </>}
    {view === 'Lead data' && <>
      <div className="card"><h2>Find company leads</h2><label className="field">Describe the companies<input className="in" value={query} onChange={e => changed(() => setQuery(e.target.value))} placeholder="Small software teams in London" /></label><label className="field">Contact roles (comma-separated, up to three)<input className="in" value={leadRoles} onChange={e => changed(() => setLeadRoles(e.target.value))} placeholder="Head of sales" /></label><label className="field">Stated basis for contact research<select className="in" value={leadBasis} onChange={e => changed(() => setLeadBasis(e.target.value))}><option value="">Choose your basis</option><option value="consent">Consent</option><option value="legitimate_interest">Legitimate interest</option><option value="contract">Contract</option></select></label><button className="btn" disabled={busy || query.trim().length < 10 || !leadRoles.trim() || leadRoles.split(',').filter(r => r.trim()).length > 3 || !leadBasis} onClick={() => void searchLeads()}>Review lead search</button><button className="btn" disabled={busy || !query.trim()} onClick={() => void run('companies.discovery-search', { query, limit: 20 })}>Discover companies</button></div>
      <div className="card"><h2>Fill contact details</h2><label className="field">Work email<input className="in" type="email" value={leadEmail} onChange={e => changed(() => setLeadEmail(e.target.value))} /></label><button className="btn" disabled={busy || !leadEmail.trim()} onClick={() => void run('people.enrich', { email: leadEmail })}>Find contact details</button><button className="btn" disabled={busy || !leadEmail.trim()} onClick={() => void run('people.email.verify', { email: leadEmail })}>Verify email</button><label className="field">Company domain<input className="in" value={leadDomain} onChange={e => changed(() => setLeadDomain(e.target.value))} /></label><button className="btn" disabled={busy || !leadDomain.trim()} onClick={() => void run('companies.enrich', { domain: leadDomain })}>Find company details</button></div>
    </>}
    {view === 'Incoming events' && <div className="card"><h2>Booking and contact events</h2><p>Read events from your connected app triggers.</p><label className="field">Find a trigger<input className="in" value={query} onChange={e => changed(() => setQuery(e.target.value))} /></label><button className="btn" disabled={busy} onClick={() => void run('apps.triggers.list', { query, limit: 50 })}>Find triggers</button><button className="btn" disabled={busy} onClick={() => void run('apps.trigger-deployments.list', { limit: 50 })}>Show connected triggers</button><Pin value="Locked" /><p>Creating a trigger is unavailable. Existing triggers remain readable.</p></div>}
    {result && <div className="card"><h2>Your result</h2>{connectedFiles(result.output).map((file,index) => <div key={index}>{file.stored === false ? <p>The action ran, but this file copy was not saved. Do not run the action again.</p> : <button className="btn" disabled={busy} onClick={() => void readFile(file.fileId!)}>Read downloaded file {index + 1} as text (uses credits)</button>}</div>)}{fileText !== undefined && <pre className="whitespace-pre-wrap">{fileText}</pre>}{connectedDisplayURL(result) && <a className="btn" href={connectedDisplayURL(result)} target="_blank" rel="noopener noreferrer">Open result</a>}
      {Array.isArray(result.domains) && (result.domains as Row[]).map((row,i)=><p key={i}>{String(row.domain??'')} — {row.available===true?'Available':row.available===false?'Unavailable':'Availability unknown'}</p>)}
      {typeof result.address === 'string' && <p>{result.address}</p>}
      {typeof result.domain === 'string' && <p>{result.domain}</p>}
      {typeof result.status === 'string' && <p>{plainLabel(result.status)}</p>}
      {typeof result.localRegistration === 'string' && <p>{result.localRegistration}</p>}
      {Array.isArray(result.dnsRecords) && <table><thead><tr><th>Type</th><th>Name</th><th>Value</th></tr></thead><tbody>{(result.dnsRecords as Row[]).map((row, i) => <tr key={i}><td>{String(row.type ?? '')}</td><td>{String(row.name ?? '')}</td><td>{String(row.value ?? '')}</td></tr>)}</tbody></table>}
      {typeof result.zoneFile === 'string' && <pre>{result.zoneFile}</pre>}
      {typeof result.url === 'string' && result.url.startsWith('https://') && <a className="btn" href={result.url} rel="noreferrer">Open setup instructions</a>}
      {result.inputSchema!==null && typeof result.inputSchema==='object' && <div><h2>Trigger fields</h2>{Object.entries((result.inputSchema as Schema).properties??{}).map(([key,field])=><p key={key}>{plainLabel(field.title)||key.replaceAll('_',' ')}{(result.inputSchema as Schema).required?.includes(key)?' — required':''}</p>)}</div>}
      {rowList(result).map((row, index) => <div className="tile" key={index}>{plainLabel(row.name ?? row.description ?? row.subject ?? row.title) || `Result ${index + 1}`}
       {view==='Incoming events' && typeof row.key==='string' && <button className="btn" disabled={busy} onClick={()=>void run('apps.trigger.get',{key:row.key})}>Read trigger fields</button>}
       {view==='Incoming events' && typeof row.deploymentId==='string' && <><button className="btn" disabled={busy} onClick={()=>void run('apps.trigger-deployment.get',{deploymentId:row.deploymentId})}>Read connected trigger</button><button className="btn" disabled={busy} onClick={()=>void run('apps.trigger-deployment-events.list',{deploymentId:row.deploymentId,limit:20,includePayload:false})}>Read recent events</button><button className="btn" disabled={busy} onClick={()=>{if(window.confirm('Disconnect this trigger?'))void run('apps.trigger-deployment.delete',{deploymentId:row.deploymentId});}}>Disconnect trigger</button></>}
      </div>)}
      {Array.isArray(result.leads) && (result.leads as Row[]).map((lead,index) => <div className="tile" key={index}>{detailRows(lead).map(([label,value],i) => <p key={i}><b>{label}</b>: {value}</p>)}{typeof lead.emailStatus === 'string' && <p>Email check: {plainLabel(lead.emailStatus)}</p>}{!lead.email && <p>Work email unavailable</p>}</div>)}
      {result.completeness === 'partial' && <p>Some details could not be checked.</p>}
      {typeof result.text === 'string' && <p>{plainLabel(result.text)}</p>}
      {detailRows(result).map(([label,value],i)=><p key={i}><b>{label}</b>: {value}</p>)}
    </div>}
  </section>;
}
