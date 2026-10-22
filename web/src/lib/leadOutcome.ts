import { capability, capabilityEstimate, outcomes, OutcomeError } from './pramaan';
import type { OutcomeBasis, OutcomeEstimate } from '@pramaan/sdk';

type Row = Record<string, unknown>;
export type LeadSearchInput = { description: string; roles: string[]; basis: OutcomeBasis };
export type LeadPlan = {
  source: 'outcome' | 'capability'; input: LeadSearchInput; estimate?: OutcomeEstimate;
  priceMaxMicro: number; priceTypicalMicro?: number; steps: string[]; created: number; key: string;
};
const deps = { outcomes, capability, capabilityEstimate };
export type LeadSearchClient = typeof deps;
const usedPlans = new WeakSet<LeadPlan>();
const opts = { max: 5, verifyEmails: true, fit: false };
const unavailable = (e: unknown) => e instanceof OutcomeError && e.code === 'upstream_unavailable' && e.status === 502 && e.effect === 'none';
async function fallbackPlan(input: LeadSearchInput, client: LeadSearchClient): Promise<LeadPlan> {
  const quote = await client.capabilityEstimate('companies.search', { query: input.description, limit: 5 });
  if (!quote.canRun || !Number.isFinite(quote.priceMaxMicro) || quote.priceMaxMicro < 0) throw new Error('Company search cannot run within your current balance or limits.');
  return { source: 'capability', input, priceMaxMicro: quote.priceMaxMicro, priceTypicalMicro: quote.priceTypicalMicro, steps: ['Company search only; contact matching is unavailable.'], created: Date.now(), key: crypto.randomUUID() };
}
export async function prepareLeadSearch(input: LeadSearchInput, client = deps): Promise<LeadPlan> {
  // Snapshot once so editing the page cannot change the approved request.
  input = JSON.parse(JSON.stringify(input)) as LeadSearchInput;
  try {
    const detail = await client.outcomes.get('lead.find');
    if (!detail.available) return fallbackPlan(input, client);
    const estimate = await client.outcomes.estimate('lead.find', { description: input.description, roles: input.roles }, { options: opts, basis: input.basis });
    if (!estimate.ready || !estimate.canRun || !estimate.plan || !Number.isFinite(estimate.priceMaxMicro) || estimate.priceMaxMicro < 0) throw new Error('Lead search needs valid inputs, a stated basis and sufficient balance before it can run.');
    return { source: 'outcome', input, estimate, priceMaxMicro: estimate.priceMaxMicro, priceTypicalMicro: estimate.priceTypicalMicro, steps: estimate.steps.filter(s => s.status === 'will_run').map(s => s.label), created: Date.now(), key: crypto.randomUUID() };
  } catch (e) { if (unavailable(e)) return fallbackPlan(input, client); throw e; }
}
export type LeadRun = { result?: Row; fallback?: LeadPlan };
export async function runLeadSearch(plan: LeadPlan, client = deps, onJob?: (id: string) => void): Promise<LeadRun> {
  if (usedPlans.has(plan)) throw new Error('This request was already submitted. Check its status before trying again.');
  usedPlans.add(plan);
  if (Date.now() - plan.created >= 15 * 60_000) throw new Error('The estimate expired. Review a new estimate.');
  if (plan.source === 'capability') return { result: await client.capability<Row>('companies.search', { query: plan.input.description, limit: 5 }, plan.key) };
  // Availability may change while the approval dialog is open.
  try {
    if (!(await client.outcomes.get('lead.find')).available) return { fallback: await fallbackPlan(plan.input, client) };
    const report = await client.outcomes.run('lead.find', { description: plan.input.description, roles: plan.input.roles }, { options: opts, basis: plan.input.basis, plan: plan.estimate!.plan, idempotencyKey: plan.key }, ...(onJob ? [onJob] as const : []));
    // Never fall back or retry a failed/partial-effect report: it may already have acted.
    if (report.steps?.some(step => step.effect === 'unknown')) throw new Error('A step has an unknown result. Check before retrying; this run will not be repeated automatically.');
    if (report.status !== 'done') throw new Error('The run did not finish. Check its recorded steps before trying again.');
    const result = report.result as Row | undefined;
    if (!result) throw new Error('The run report is incomplete. Check before trying again.');
    return { result: { ...result, completeness: report.completeness, missing: report.missing } };
  } catch (e) { if (unavailable(e)) return { fallback: await fallbackPlan(plan.input, client) }; throw e; }
}
