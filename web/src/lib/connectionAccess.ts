// Required consent notices describe connection grants, never a restricted access level.
const scopes: Record<string, string> = {
  gmail: 'Connecting gives this app full access to your Gmail: it can read, send and permanently delete mail, and read your profile and contacts.',
  googledrive: 'Connecting gives this app full access to your Google Drive: it can read, change, share and delete every file.',
  googlecalendar: 'Connecting gives this app read and write access to your Google Calendar.',
  slack: 'Connecting gives this app read and write access to your Slack account.',
  hubspot: 'Connecting gives this app read and write access to your HubSpot account.',
  notion: 'Connecting gives this app access to the Notion pages you select in its own picker.',
};
const unknown: Record<string, string> = {
  googlesheets:'Google Sheets',googledocs:'Google Docs',google_search_console:'Google Search Console',google_analytics:'Google Analytics',airtable:'Airtable',linear:'Linear',salesforce:'Salesforce',calendly:'Calendly',outlook:'Outlook',github:'GitHub',
};
// These are notice aliases only; the exact catalogue slug is sent to the API unchanged.
const aliases: Record<string,string> = {google_drive:'googledrive',google_calendar:'googlecalendar',google_sheets:'googlesheets',google_docs:'googledocs'};
export function connectionAccessNotice(app: unknown, usage: 'actions'|'mailbox' = 'actions'): string {
  const raw=typeof app==='string'?app.toLowerCase():'', slug=aliases[raw]??raw;
  const grant=scopes[slug]??`Connecting may give this app full access to your ${unknown[slug]??(usage==='mailbox'?'email':'connected app')} account.`;
  const use=usage==='mailbox'?'Cold Email uses this connection to send scheduled mail and replies, and sync mailbox messages for your workspace.':'Cold Email uses this connection only for the catalogue actions you select or enable and the events you configure; changes require confirmation or an automation you have enabled.';
  return `${grant} ${use} The connector chooses the requested access; Cold Email cannot narrow it today. Review the consent screen before continuing. The hosted link lasts about 10 minutes.`;
}
export function redirectToConnection(raw: unknown): void {
  if(typeof raw!=='string')return;
  try {const url=new URL(raw);if(url.protocol==='https:'&&!url.username&&!url.password)window.location.assign(url.href);} catch { /* A malformed link is not followed. */ }
}
