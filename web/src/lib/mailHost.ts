// Display names for a mailbox's host and sign-in method, and the one-line
// neutral label the mailbox list and drawer show.
import type { MailAuthMethod, MailHost } from "@/lib/api/models/app/emails/MailboxImport";

export const MAIL_HOST_LABELS: Record<Exclude<MailHost, "">, string> = {
    google_workspace: "Managed mailbox",
    gmail: "Managed mailbox",
    microsoft365: "Managed mailbox",
    outlook: "Managed mailbox",
    zoho: "Managed mailbox",
    yahoo: "Managed mailbox",
    aol: "Managed mailbox",
    icloud: "Managed mailbox",
    fastmail: "Managed mailbox",
    godaddy: "Managed mailbox",
    namecheap: "Managed mailbox",
    ionos: "Managed mailbox",
    hostinger: "Managed mailbox",
    ovh: "Managed mailbox",
    migadu: "Managed mailbox",
    purelymail: "Managed mailbox",
    rackspace: "Managed mailbox",
    yandex: "Managed mailbox",
    gmx: "Managed mailbox",
    proton: "Managed mailbox",
    other: "Managed mailbox",
};

export const AUTH_METHOD_LABELS: Record<Exclude<MailAuthMethod, "">, string> = {
    password: "Password",
    app_password: "App password",
    oauth: "OAuth",
    delegated: "Admin grant",
};

// The connect type the mailbox list showed before hosts were detected.
const PROVIDER_LABELS: Record<string, string> = {
    gmail: "Managed mailbox",
    outlook: "Managed mailbox",
    smtp_imap: "SMTP/IMAP",
};

export function mailHostLabel(host?: string | null): string {
    if (!host) return "";
    return MAIL_HOST_LABELS[host as Exclude<MailHost, "">] ?? "Managed mailbox";
}

export function authMethodLabel(method?: string | null): string {
    if (!method) return "";
    return AUTH_METHOD_LABELS[method as Exclude<MailAuthMethod, "">] ?? method;
}

/** Local delivery family; never a displayed brand or artwork selector. */
export function mailHostFamily(host?: string | null): "google" | "microsoft" | null {
    if (host === "google_workspace" || host === "gmail") return "google";
    if (host === "microsoft365" || host === "outlook") return "microsoft";
    return null;
}

/** Neutral host/auth label, including unknown imported values. */
export function mailboxConnectionLabel(box: { mail_host?: string | null; auth_method?: string | null; provider?: string | null }): string {
    const host = mailHostLabel(box.mail_host);
    if (!host) return PROVIDER_LABELS[box.provider ?? ""] ?? (box.provider ? "Managed mailbox" : "");
    const method = authMethodLabel(box.auth_method);
    return method ? `${host} · ${method}` : host;
}
