import { domainCapability, capability, capabilityEstimate } from './pramaan';
export type DomainPrice = { amount: number; currency: 'USD' };
export type DomainQuote = { domain: string; action: 'register'; years: number; price: DomainPrice; confirmation: string };
export type Registrant = { firstName: string; lastName: string; address: string; city: string; region: string; postalCode: string; country: string; phone: string; email: string };
export type Registration = { domain: string; ref: string; state: string; verification?: { state: string } };
export type MailDomain = { domain: string; ref?: string; status: string; dnsRecords: { type: string; name: string; value: string; priority?: number }[] };
export const domainClient = { domain: domainCapability, mail: capability, mailEstimate: capabilityEstimate };
export const allowedDomainRole = (role?: string) => role === 'owner' || role === 'admin';
export function purchasePrice(price: DomainPrice) { return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(price.amount / 100); }
export type PurchasePlan = { input: DomainQuote & { registrant: Registrant }; key: string; created: number; submitted: boolean };
export function preparePurchase(quote: DomainQuote, registrant: Registrant): PurchasePlan {
  if (!quote.confirmation || quote.action !== 'register' || !quote.domain || !Number.isInteger(quote.price?.amount) || quote.price.amount < 0 || quote.price.currency !== 'USD') throw new Error('This domain needs a current purchase quote.');
  return { input: structuredClone({ ...quote, registrant }), key: crypto.randomUUID(), created: Date.now(), submitted: false };
}
export async function confirmPurchase(plan: PurchasePlan, role: string | undefined, confirmed: boolean, client = domainClient) {
  if (!allowedDomainRole(role)) throw new Error('Only a workspace owner or admin can buy a domain.');
  if (!confirmed) throw new Error('Confirm the real, non-refundable purchase price first.');
  if (plan.submitted) throw new Error('This purchase was already submitted. Check its status.');
  if (Date.now() - plan.created > 15 * 60_000) throw new Error('The quote expired. Review a new quote.');
  plan.submitted = true;
  const { action: _action, ...input } = plan.input;
  return client.domain<Registration>('domains.registration.create', { ...input, privacy: true }, plan.key);
}
export async function recoverPurchase(domain: string, client = domainClient): Promise<Registration | undefined> {
  // Create may have lost its reply before returning a ref. Discover a scoped
  // owned reference, then get it; an absent listing never permits re-purchase.
  let cursor: string | undefined;
  for (let page = 0; page < 20; page++) {
    const list: { results: Registration[]; nextCursor?: string } = await client.domain('domains.registration.list', { limit: 100, ...(cursor ? { cursor } : {}) });
    const row = list.results?.find(row => row.domain === domain && row.ref);
    if (row) return { ...row, ...await client.domain<Registration>('domains.registration.get', { ref: row.ref }), ref: row.ref };
    if (!list.nextCursor) return;
    cursor = list.nextCursor;
  }
  throw new Error('The domain list is still incomplete. Check again later.');
}
export async function applyMailDNS(registration: Registration, mail: MailDomain, client = domainClient) {
  const records = mail.dnsRecords;
  if (!records?.length || records.length > 20 || mail.domain !== registration.domain) throw new Error('Review the mail DNS records before applying them.');
  const result = await client.domain<{ results: { status: string }[] }>('domains.dns-records.apply', { ref: registration.ref, set: records.map(record => ({ ...record, ttl: 300 })), delete: [] });
  if (result.results?.length !== records.length || result.results.some(row => !['created', 'updated', 'unchanged'].includes(row.status))) throw new Error('Some DNS records were not applied. Read the domain records before changing anything else.');
  return client.mail<MailDomain>('email.domain.get', { domain: mail.domain });
}
