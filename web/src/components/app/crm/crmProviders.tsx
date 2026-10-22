// The connected CRMs a workspace can run on, and what each calls things. Every
// CRM surface reads this through useCrmProvider, so connected and connected mode
// share one set of screens with each provider's own logo and words.

import { ExternalLink } from "lucide-react";
import { cn } from "@/lib/utils";
import type { CRMExternalRef } from "@/lib/api/models/app/crm/CRMProvider";

export type ExternalCrm = "native";

export interface CrmInfo {
    id: ExternalCrm;
    name: string;
    settingsPath: string;
    // Tailwind classes in the provider's colour.
    tint: string;
    tintText: string;
    hoverText: string;
    hoverBg: string;
    border: string;
    words: {
        contact: string;
        contacts: string;
        company: string;
        owner: string;
        owners: string;
        task: string;
        tasks: string;
        list: string;
        lists: string;
        property: string;
        properties: string;
        // What a contact's lifecycle stage is called ("Label" in connected).
        lifecycle: string;
        lifecycles: string;
        // connected has no lead status.
        leadStatus: string | null;
        amount: string;
        closeDate: string;
    };
    // connected closes a deal by moving it to a won or lost stage; connected
    // keeps the stage and marks the deal won or lost.
    wonLostAreStages: boolean;
}

export const CRM_INFO: Record<ExternalCrm, CrmInfo> = {
 native: { id: "native", name: "Cold Email CRM", settingsPath: "/app/integrations", tint: "bg-sky-50", tintText: "text-sky-700", hoverText: "hover:text-sky-700", hoverBg: "hover:bg-sky-50", border: "border-sky-200", words: {contact:"contact",contacts:"contacts",company:"company",owner:"owner",owners:"owners",task:"task",tasks:"tasks",list:"list",lists:"lists",property:"field",properties:"fields",lifecycle:"Stage",lifecycles:"stages",leadStatus:"Lead status",amount:"Amount",closeDate:"Close date"}, wonLostAreStages: true }
};
export function crmInfo(_provider?: string | null): CrmInfo { return CRM_INFO.native; }
export function CrmMark({className, title}: {provider?: string | null;className?:string;title?:string}) {return <ExternalLink className={className} aria-label={title ?? "CRM"}/>;}

// A small provider chip for section headers and pickers.
export function CrmBadge({ provider, className, label }: { provider?: string | null; className?: string; label?: string }) {
    const info = crmInfo(provider);
    return (
        <span
            className={cn(
                "inline-flex items-center gap-1 h-5 px-1.5 rounded-md text-[10.5px] font-medium shrink-0",
                info.tint,
                info.tintText,
                className,
            )}
        >
            <CrmMark provider={info.id} className="w-3 h-3" />
            {label ?? info.name}
        </span>
    );
}

// "Open in connected / connected" for a record. Renders nothing without a link.
// Stops propagation so it never also opens the row it sits in.
export function OpenInCrm({
    url,
    external,
    provider,
    label,
    compact = false,
    className,
}: {
    url?: string;
    external?: CRMExternalRef;
    provider?: string | null;
    label?: string;
    compact?: boolean;
    className?: string;
}) {
    const info = crmInfo(provider ?? external?.provider);
    const href = url ?? external?.url;
    if (!href) return null;
    const text = label ?? `Open in ${info.name}`;
    return (
        <a
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            onClick={(e) => e.stopPropagation()}
            onMouseDown={(e) => e.stopPropagation()}
            title={text}
            aria-label={text}
            className={cn(
                "inline-flex items-center gap-1 text-[12px] text-slate-600 transition-colors shrink-0",
                info.hoverText,
                info.hoverBg,
                compact ? "h-6 w-6 justify-center rounded-md" : "h-7 px-2 rounded-md",
                className,
            )}
        >
            <CrmMark provider={info.id} className="w-3.5 h-3.5" />
            {!compact && <span>{text}</span>}
            {!compact && <ExternalLink className="w-3 h-3 opacity-60" />}
        </a>
    );
}

// "Synced with connected · 2m ago", for footers and panel headers.
export function CrmSyncedAt({ at, provider, className }: { at?: Date | string; provider?: string | null; className?: string }) {
    if (!at) return null;
    const info = crmInfo(provider);
    const d = typeof at === "string" ? new Date(at) : at;
    return (
        <span className={cn("inline-flex items-center gap-1 text-[11px] text-slate-400", className)}>
            <CrmMark provider={info.id} className="w-3 h-3 opacity-80" />
            Synced with {info.name} · {timeAgo(d)}
        </span>
    );
}

function timeAgo(d: Date): string {
    const s = Math.max(0, Math.round((Date.now() - d.getTime()) / 1000));
    if (s < 45) return "just now";
    const m = Math.round(s / 60);
    if (m < 60) return `${m}m ago`;
    const h = Math.round(m / 60);
    if (h < 24) return `${h}h ago`;
    return `${Math.round(h / 24)}d ago`;
}
