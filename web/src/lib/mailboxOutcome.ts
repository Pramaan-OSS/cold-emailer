import { outcomes } from './pramaan';

type Client = Pick<typeof outcomes, 'get' | 'estimate' | 'run'>;
export type MailboxPlan = { input: Record<string, unknown>; estimate: Awaited<ReturnType<Client['estimate']>>; key: string; created: number; submitted: boolean };
export async function prepareMailbox(check = false, client: Client = outcomes): Promise<MailboxPlan> {
  if (!(await client.get('mailbox.connect')).available) throw new Error('Mailbox connection is not available yet.');
  const input = check ? { check: true } : {};
  const estimate = await client.estimate('mailbox.connect', input);
  if (!estimate.ready || !estimate.canRun || !estimate.plan || !Number.isFinite(estimate.priceMaxMicro)) throw new Error('Mailbox connection cannot run within your current balance or limits.');
  return { input, estimate, key: crypto.randomUUID(), created: Date.now(), submitted: false };
}
export async function runMailbox(plan: MailboxPlan, client: Client = outcomes) {
  if (plan.submitted) throw new Error('This connection request was already submitted. Check its status before trying again.');
  if (Date.now() - plan.created >= 15 * 60_000) throw new Error('The estimate expired. Review a new estimate.');
  plan.submitted = true;
  const report = await client.run('mailbox.connect', plan.input, { plan: plan.estimate.plan, idempotencyKey: plan.key });
  const result = report.result as Record<string, unknown> | undefined;
  if (!result) throw new Error('The connection result is unknown. Check its status before trying again.');
  return { result, complete: report.status === 'done' && !report.steps?.some(step => step.effect === 'unknown') };
}
