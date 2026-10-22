import { Pramaan, PramaanError, type PramaanOptions, type OutcomeRun, type OutcomeRunStep, type OutcomeRunOptions } from '@pramaan/sdk';
import { API_BASE_URL } from './information';
import { ensureValidToken } from './api/client/Request';

// The browser uses its app session. The backend alone holds PRAMAAN_API_KEY.
export async function capability<T = Record<string, unknown>>(id: string, input: unknown, key: string = crypto.randomUUID()): Promise<T> {
  const session = (await ensureValidToken()).access_token;
  if (!session) throw new Error('Sign in before using this feature.');
  const client = createPramaanClient({ apiKey: session, baseUrl: `${API_BASE_URL}/auth/pramaan`, maxRetries: 0 });
  try { return await client.call(id, input, { idempotencyKey: key }) as T; }
  catch (error) {
    if (error instanceof PramaanError && error.reason === "reply_incomplete") throw new Error("The reply is incomplete. Retry with a larger reply limit.");
    if (error instanceof PramaanError && error.reason === "reply_limit") throw new Error("The reply limit is too small. Increase maxTokens in the request settings before trying again.");
    if (error instanceof PramaanError && (error.code === 'upstream_unavailable' || error.status === 501)) throw new Error(error.effect === 'none' ? 'This feature is not available yet.' : 'This feature is not available yet. Its result may be unknown; check before retrying.');
    throw new Error('The action could not complete. Check your connection and action fields, then try again.');
  }

}
export function plainLabel(text: unknown): string {
  if (typeof text !== 'string') return '';
  // Connected-app names returned by discovery are allowed; strip links and prices.
  return text.replace(/https?:\/\/\S+/g, '').replace(/\$\d[\d.,]*/g, '').trim();
}

// Outcome errors preserve refusal codes without exposing upstream messages.
export class OutcomeError extends Error {
  readonly code: string;
  readonly status?: number;
  readonly effect?: 'none';
  constructor(code: string, status?: number, effect?: 'none') {
    super(code === 'upstream_unavailable' ? 'This feature is not available yet.' : code === 'upstream_timeout' ? 'The run timed out. Check its recorded steps before retrying; some results may be unknown.' : code === 'rate_limited' ? 'Pramaan is busy. Try again later.' : code === 'conflict' ? 'The plan changed or expired. Review a new estimate.' : code === 'forbidden' ? 'Approval is required. Review the estimate first.' : 'The run could not complete. Check its status before trying again.');
    this.code = code; this.status = status; this.effect = effect;
  }
}
async function sdk(onJob?: (id: string) => void) {
  const session = (await ensureValidToken()).access_token;
  if (!session) throw new Error('Sign in before using this feature.');
  return createPramaanClient({ apiKey: session, baseUrl: `${API_BASE_URL}/auth/pramaan`, maxRetries: 0, fetch: async (url, init) => {
    const response = await pollingFetch(url, init);
    if (onJob && init.method === 'POST' && /\/outcomes\/[^/]+$/.test(new URL(url).pathname) && response.status === 202) {
      const body = await response.clone().json().catch(() => ({}));
      if (typeof body.job_id === 'string') onJob(body.job_id);
    }
    return response;
  } });
}
async function outcomeCall<T>(call: (client: Pramaan) => Promise<T>, onJob?: (id: string) => void): Promise<T> {
  try { return await call(await sdk(onJob)); }
  catch (error) {
    if (error instanceof PramaanError) throw new OutcomeError(error.code, error.status, error.effect);
    throw new OutcomeError('internal');
  }
}
export const outcomes = {
  list: () => outcomeCall(client => client.outcomes.list()),
  get: (id: string) => outcomeCall(client => client.outcomes.get(id)),
  estimate: (id: string, input: unknown, opts: OutcomeRunOptions = {}) => outcomeCall(client => client.outcomes.estimate(id, input, opts)),
  cancel: (id: string) => outcomeCall(client => client.jobs.cancel(id)),
  run: (id: string, input: unknown, opts: OutcomeRunOptions = {}, onJob?: (id: string) => void) => outcomeCall(client => client.outcomes.run(id, input, opts), onJob),
};
export const capabilityEstimate = (id: string, input: unknown) => outcomeCall(client => client.capabilities.estimate(id, input));

// Retrying a rate-limited job read cannot start another outcome. POSTs are sent once.
export async function pollingFetch(url: string, init: RequestInit, sleep = (ms: number) => new Promise<void>(resolve => setTimeout(resolve, ms))): Promise<Response> {
  for (let attempt = 0; ; attempt++) {
    const response = await fetch(url, init);
    if (init.method !== 'GET' || !new URL(url).pathname.includes('/jobs/') || response.status !== 429) return response;
    if (attempt >= 3) throw new PramaanError('upstream_timeout', 'The job is still waiting. Check its status before trying again.');
    await sleep(2000);
  }
}
export function connectedResultRows(value: unknown): Record<string, unknown>[] {
  const object = (v: unknown): Record<string, unknown> | undefined => v && typeof v === 'object' && !Array.isArray(v) ? v as Record<string, unknown> : undefined;
  const root = object(value), output = object(root?.output);
  const rows = Array.isArray(root?.results) ? root.results : Array.isArray(output?.results) ? output.results : Array.isArray(root?.output) ? root.output : [];
  return rows.filter((row): row is Record<string, unknown> => !!object(row));
}
export function connectedDisplayURL(value: unknown): string | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return;
  const root = value as Record<string, unknown>, output = root.output;
  const raw = root.display_url ?? (output && typeof output === 'object' && !Array.isArray(output) ? (output as Record<string, unknown>).display_url : undefined);
  if (typeof raw !== 'string') return;
  try { const url = new URL(raw); if (url.protocol === 'https:' && !url.username && !url.password) return url.href; } catch { /* No link for malformed output. */ }
}

// Domain purchases preserve no-effect vs uncertain errors and never retry a POST.
export class DomainError extends Error {
  constructor(public readonly code: string, public readonly effect: string = 'unknown') {
    super(code === 'unavailable' ? 'Domain purchase is not available yet; use a domain you own' : code === 'forbidden' ? 'Only a workspace owner or admin can buy a domain, after reviewing its quote.' : effect === 'none' ? 'The action was not sent. Review a new quote before trying again.' : 'The result is unknown. Check the domain status; do not submit the purchase again.');
  }
}
export async function domainCapability<T = Record<string, unknown>>(id: string, input: unknown, key: string = crypto.randomUUID()): Promise<T> {
  try { return await (await sdk()).call(id, input, { idempotencyKey: key }) as T; }
  catch (error) {
    if (error instanceof PramaanError) {
      if ((error.code === 'upstream_unavailable' && error.effect === 'none') || error.status === 501) throw new DomainError('unavailable', 'none');
      throw new DomainError(error.code, error.effect);
    }
    throw new DomainError('unknown');
  }
}

export type CatalogueInput = { query: string; app?: string; cursor?: string; limit?: number };
export type CataloguePage = { results: Record<string, unknown>[]; nextCursor?: string };
// In-memory only: one organisation/session/query page, never an automatic crawl.
export function createCatalogueCache(fetchPage: (input: CatalogueInput) => Promise<CataloguePage>) {
  const pages = new Map<string, { created: number; promise: Promise<CataloguePage> }>();
  return async (input: CatalogueInput, scope: string): Promise<CataloguePage> => {
    input = { query: input.query.trim(), ...(input.app ? { app: input.app } : {}), limit: 50, ...(input.cursor ? { cursor: input.cursor } : {}) };
    if (!input.query) throw new Error('Describe the action first.');
    const key = JSON.stringify([scope, input]);
    const cached = pages.get(key);
    if (cached && Date.now() - cached.created < 24 * 60 * 60_000) return structuredClone(await cached.promise);
    const promise = fetchPage(input);
    pages.set(key, { created: Date.now(), promise });
    if (pages.size > 128) pages.delete(pages.keys().next().value!);
    try { return structuredClone(await promise); } catch (error) { pages.delete(key); throw error; }
  };
}
const cachedCatalogue = createCatalogueCache(input => capability<CataloguePage>('apps.catalog.search', input));
export async function cataloguePage(input: CatalogueInput): Promise<CataloguePage> {
  const session = await ensureValidToken();
  if (!session.access_token) throw new Error('Sign in before searching.');
  // Workspace selection can change while the app-session token stays the same.
  const { useAppStore } = await import('@/stores');
  const scope = `${useAppStore.getState().currentOrganization?.id ?? 'none'}:${session.access_token}`;
  return cachedCatalogue(input, scope);
}
export function connectedFiles(value: unknown): { fileId?: string; stored?: false }[] {
  const files: { fileId?: string; stored?: false }[] = [], seen = new Set<string>();
  function walk(v: unknown, depth: number) {
    if (!v || typeof v !== 'object' || depth > 12) return;
    if (Array.isArray(v)) { v.forEach(item => walk(item, depth + 1)); return; }
    const row = v as Record<string, unknown>;
    if (row.stored === false) { files.push({ stored: false }); return; }
    if (typeof row.fileId === 'string' && row.fileId.startsWith('file_') && !seen.has(row.fileId)) { seen.add(row.fileId); files.push({ fileId: row.fileId }); }
    Object.values(row).forEach(item => walk(item, depth + 1));
  }
  walk(value, 0); return files;
}
export async function parseConnectedFile(fileId: string) {
  if (!fileId.startsWith('file_')) throw new Error('This file has no saved platform handle.');
  return capability<{ text: string; truncated: boolean }>('document.parse', { fileId, ocr: false });
}

// The returned address serves sign-in requests; it is not a hosted page.
type ManagedAuthReply = {error?: unknown};
export interface ManagedAuthClient {
 signIn: { social(input:{provider:string;callbackURL:string}):Promise<ManagedAuthReply>; email(input:{email:string;password:string;callbackURL:string}):Promise<ManagedAuthReply> };
 signUp: { email(input:{email:string;password:string;name:string;callbackURL:string}):Promise<ManagedAuthReply> };
 token():Promise<{data?:{token?:string}|null;error?:unknown}>;
}
export async function managedSignInClient(authUrl: string): Promise<ManagedAuthClient> {
  const module = await import('better-auth/client');
  const createClient = module.createAuthClient as unknown as (options: {baseURL:string;fetchOptions:{retry:0}}) => ManagedAuthClient;
  return createClient({ baseURL: authUrl, fetchOptions: { retry: 0 } });
}
export async function managedSignInToken(authUrl: string): Promise<string> {
  try {
    const callback = new URL(window.location.href);
    const verifierKey = 'neon_auth_session_verifier';
    const verifier = callback.searchParams.get(verifierKey);
    if (verifier !== null) {
      // Consume the one-time callback in this browser's cookie partition before
      // asking for a token. Never retain the verifier in the address or retry it.
      callback.searchParams.delete(verifierKey);
      window.history.replaceState(window.history.state, '', callback);
      if (!verifier || verifier.length > 4096) throw new Error();
      const endpoint = new URL(authUrl.replace(/\/$/, '') + '/get-session');
      endpoint.searchParams.set(verifierKey, verifier);
      const session = await fetch(endpoint, {method:'GET',credentials:'include',redirect:'error'});
      if (!session.ok) throw new Error();
      const value = await session.json() as {session?:unknown;error?:unknown}|null;
      if (!value?.session || value.error) throw new Error();
    }
    const client = await managedSignInClient(authUrl);
    const { data, error } = await client.token();
    if (error || !data?.token) throw new Error();
    return data.token;
  } catch { throw new Error('Managed sign-in could not be verified. Sign in again.'); }
}

// Preserve long job waits and failed reports, which SDK 0.1.0 does not retain.
export function createPramaanClient(options: PramaanOptions): Pramaan {
  const client = new Pramaan({ ...options, maxRetries: 0 });
  const wait = client.jobs.wait;
  client.jobs.wait = (id, options = {}) => wait(id, { timeoutMs: 36 * 60_000, ...options });
  client.outcomes.run = (async (id: string, input: unknown, opts: OutcomeRunOptions = {}) => {
    const started = await client.outcomes.start(id, input, opts);
    if (!("id" in started)) return started;
    const job = await client.jobs.wait(started.id);
    if (job.result && typeof job.result === "object") return toOutcomeRun(job.result);
    throw new PramaanError(job.error?.code ?? "internal", job.error?.message ?? `The run ended with status "${job.status}".`, { effect: job.error?.effect });
  }) as typeof client.outcomes.run;
  return client;
}
type Raw = Record<string, unknown>;
function toOutcomeRun(body: unknown): OutcomeRun {
  const raw = (body ?? {}) as Raw;
  const cost = raw.cost as Raw | undefined;
  if (typeof raw.outcome !== "string" || typeof raw.status !== "string" || !cost) {
    throw new PramaanError("internal", "The API returned an unexpected run shape.");
  }
  return {
    outcome: raw.outcome,
    version: Number(raw.version ?? 1),
    plan: String(raw.plan),
    status: raw.status as OutcomeRun["status"],
    ...(typeof raw.completeness === "string"
      ? { completeness: raw.completeness as NonNullable<OutcomeRun["completeness"]> }
      : {}),
    ...("result" in raw ? { result: raw.result } : {}),
    evidence: (raw.evidence ?? {}) as Record<string, unknown>,
    missing: (raw.missing ?? []) as unknown[],
    steps: ((raw.steps ?? []) as Raw[]).map(
      (step): OutcomeRunStep => ({
        id: String(step.id),
        label: String(step.label),
        capability: String(step.capability),
        effectClass: step.effect_class as OutcomeRunStep["effectClass"],
        status: step.status as OutcomeRunStep["status"],
        calls: Number(step.calls ?? 0),
        effect: step.effect as OutcomeRunStep["effect"],
        ...(typeof step.reason === "string" ? { reason: step.reason } : {}),
        ...(Array.isArray(step.errors)
          ? { errors: step.errors as NonNullable<OutcomeRunStep["errors"]> }
          : {}),
      }),
    ),
    ...(raw.error ? { error: raw.error as NonNullable<OutcomeRun["error"]> } : {}),
    cost: {
      holdMicro: Number(cost.hold_micro ?? 0),
      chargedMicro: Number(cost.charged_micro ?? 0),
      capabilities: ((cost.capabilities ?? []) as Raw[]).map((row) => ({
        id: String(row.id),
        calls: Number(row.calls ?? 0),
        chargedMicro: Number(row.charged_micro ?? 0),
      })),
    },
  };
}
