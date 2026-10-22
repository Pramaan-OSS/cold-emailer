import type { MailboxImportOptions } from "./MailboxImport";

export type SourceStatus = "active" | "invalid";
export type SourceImportOptions = Required<Pick<MailboxImportOptions, "on_existing" | "settings">> & Pick<MailboxImportOptions, "tracking_domains" | "redirects">;

export type GrantProvider = "google" | "microsoft";

export interface GrantConfig {
    google_enabled: boolean;
    google_client_id?: string;
    google_scopes: string[];
    microsoft_enabled: boolean;
    /** Settings an operator still has to set; empty when enabled. */
    google_missing?: string[];
    microsoft_missing?: string[];
}

/**
 * How a workspace proves it controls a Workspace domain. "signin" opens `url`
 * for the administrator; the DNS record works either way.
 */
export interface GoogleGrantStart {
    method: "signin" | "dns";
    url?: string;
    state?: string;
    txt_name: string;
    txt_value: string;
}

export type GoogleGrantFinish = { state: string; code: string } | { domain: string; admin_email: string };

export interface DomainGrant {
    id: string;
    provider: GrantProvider;
    /** The Google domain, or the Microsoft tenant id. */
    tenant: string;
    admin_email?: string;
    domains: string[];
    status: SourceStatus;
    last_error?: string;
    verified_at?: Date | null;
    created_at: Date;
    mailboxes: number;
}

/** One account in a granted domain or tenant. */
export interface DirectoryUser {
    /** The address (Google) or the Graph user id (Microsoft). */
    id: string;
    email: string;
    name: string;
    /** False for a suspended or disabled account. */
    enabled: boolean;
    connected: boolean;
    email_account_id?: string;
    /** Already a mailbox here on per-mailbox sign-in; connecting moves it onto the grant, keeping its history. */
    upgrade?: boolean;
}

export interface GrantConnectRequest {
    user_ids?: string[];
    /** Every enabled user not connected yet. */
    all?: boolean;
    options: SourceImportOptions;
}

/** A mailbox still on the retiring per-mailbox Google sign-in. */
export interface MigrationMailbox {
    id: string;
    email: string;
    name: string;
    status: string;
}

/**
 * One domain's mailboxes on per-mailbox Google sign-in. "personal" is a shared
 * address like gmail.com (moves to an app password); "workspace" moves to an
 * admin grant, which `grant_id` names when the domain already has one.
 */
export interface SigninMigration {
    domain: string;
    kind: "workspace" | "personal";
    grant_id?: string;
    mailboxes: MigrationMailbox[];
}

export interface SigninMigrationList {
    data: SigninMigration[];
    total: number;
}

export interface ListResponse<T> {
    data: T[];
}

// Vendor ids the backend knows, for labels where the catalog is not loaded.


export const GRANT_PROVIDER_LABELS: Record<GrantProvider, string> = {
    google: "Managed mailbox",
    microsoft: "Managed mailbox",
};

export function vendorLabel(id?: string | null): string {
    if (!id) return "";
    return GRANT_PROVIDER_LABELS[id as GrantProvider] ?? "Imported mailbox";
}

/** The domain part of an address, lower-cased. */
export function domainOf(email: string): string {
    const at = email.lastIndexOf("@");
    return at >= 0 ? email.slice(at + 1).toLowerCase() : "";
}

/** The grant that covers an address, if any. */
export function grantFor(grants: DomainGrant[] | undefined, email: string): DomainGrant | undefined {
    const d = domainOf(email);
    if (!d || !grants) return undefined;
    return grants.find((g) => g.tenant.toLowerCase() === d || g.domains.some((x) => x.toLowerCase() === d));
}

/** A grant as people know it: the Google domain, or the Microsoft tenant's domains. */
export function grantName(g: DomainGrant): string {
    if (g.provider === "google") return g.tenant;
    return g.domains.length > 0 ? g.domains.join(", ") : g.tenant;
}
