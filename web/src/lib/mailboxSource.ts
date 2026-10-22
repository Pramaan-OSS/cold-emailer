// How a mailbox is connected: the inbox vendor it came from, the admin grant it signs in through, or its host.
import type Inbox from "@/lib/api/models/app/emails/Inbox";
import { vendorLabel } from "@/lib/api/models/app/emails/MailboxSources";
import { mailboxConnectionLabel } from "@/lib/mailHost";

export type MailboxSourceBox = Pick<Inbox, "provider" | "mail_host" | "auth_method" | "vendor">;

export interface MailboxSource {
    kind: "vendor" | "grant" | "host";
    /** The id ProviderLogo draws. */
    logo: string;
    label: string;
    title: string;
}

export function mailboxSource(box: MailboxSourceBox): MailboxSource {
    const connection = mailboxConnectionLabel(box);
    if (box.vendor) {
        const name = vendorLabel(box.vendor);
        return { kind: "vendor", logo: "mailbox", label: `via ${name}`, title: `Imported from ${name}${connection ? ` · ${connection}` : ""}` };
    }
    if (box.auth_method === "delegated") {
        return {
            kind: "grant",
            logo: "mailbox",
            label: "Admin grant",
            title: "Connected through an administrator grant",
        };
    }
    return { kind: "host", logo: "mailbox", label: connection, title: connection };
}

/** All mailbox avatars use the same neutral mark. */
export function mailboxBrand(_box: Pick<Inbox, "provider" | "mail_host">): "mailbox" {
    return "mailbox";
}
