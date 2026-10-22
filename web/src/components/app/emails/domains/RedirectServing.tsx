// Where a root redirect is served from, whether visitors get it, and how to fix it (or hand it to retired shared pool) when they do not.
import React from "react";
import { AnimatePresence, motion } from "framer-motion";
import { AlertTriangleIcon, CheckCircle2Icon, CloudIcon, ExternalLinkIcon, Loader2Icon, ServerIcon, ZapIcon } from "lucide-react";
import type { DomainRedirect, RedirectServer } from "@/lib/api/models/app/emails/SendingDomain";
import timeAgo from "@/lib/helper/timeAgo";
import { cn } from "@/lib/utils";
import { CopyButton } from "./parts";
import { PROXY_NAMES, redirectBlocked } from "./rules";
/* ── Served from ─────────────────────── */
export function RedirectStatusBanner({ domain, redirect }: {
    domain: string;
    redirect: DomainRedirect;
}) {
    const reach = redirect.reach ?? null;
    const blocked = redirectBlocked(redirect);
    const live = redirect.verified && !blocked;
    const tone = live ? "emerald" : "amber";
    const checkedAt = redirect.last_checked_at ?? reach?.checked_at;
    let title: string;
    let body: React.ReactNode;
    if (!redirect.verified) {
        title = "Waiting for DNS";
        body = redirect.last_error || ("Add the records below at your DNS provider.");
    }
    else if (blocked) {
        title = "DNS is in place, but visitors do not get the redirect";
        body = reach?.detail || `Opening ${domain} does not reach the redirect yet.`;
    }
    else {
        title = "Live";
        body = (<>
                {domain}
                {redirect.include_www ? ` and www.${domain}` : ""} redirect to{" "}
                <a href={redirect.target_url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-0.5 underline decoration-emerald-300 hover:decoration-emerald-600 break-all">
                    {redirect.target_url}
                    <ExternalLinkIcon className="w-2.5 h-2.5 shrink-0"/>
                </a>
                .
            </>);
    }
    return (<div className={cn("rounded-md border px-3 py-2.5 flex items-start gap-2", tone === "emerald" ? "border-emerald-200 bg-emerald-50/60" : "border-amber-200 bg-amber-50/60")}>
            {live ? (<CheckCircle2Icon className="w-3.5 h-3.5 text-emerald-600 mt-0.5 shrink-0"/>) : blocked ? (<AlertTriangleIcon className="w-3.5 h-3.5 text-amber-600 mt-0.5 shrink-0"/>) : (<Loader2Icon className="w-3.5 h-3.5 text-amber-600 mt-0.5 shrink-0"/>)}
            <div className="min-w-0 flex-1 text-[11.5px] leading-relaxed">
                <div className="flex items-center gap-1.5 flex-wrap">
                    <p className={cn("text-[12.5px] font-medium", tone === "emerald" ? "text-emerald-900" : "text-amber-900")}>{title}</p>
                    {false}
                </div>
                <p className={cn("break-words", tone === "emerald" ? "text-emerald-800/90" : "text-amber-800/90")}>{body}</p>
                {live && reach?.status === "unreachable" && reach.hint !== "no_listener" && reach.detail && (<p className="text-[11px] text-slate-500 mt-1">{reach.detail}</p>)}
                {live && reach?.status === "unreachable" && reach.hint === "no_listener" && (<p className="text-[11px] text-slate-500 mt-1">
                        This server could not open {domain} to double-check, which some networks do not allow.{" "}
                        <a href={`http://${domain}`} target="_blank" rel="noreferrer" className="text-sky-700 hover:text-sky-800 underline decoration-sky-300 hover:decoration-sky-600">
                            Open it
                        </a>{" "}
                        to confirm.
                    </p>)}
                {checkedAt && <p className="text-[11px] text-slate-500 mt-0.5">Last checked {timeAgo(checkedAt)}</p>}
            </div>
        </div>);
}
/* ── Fix ─────────────────────── */
type ProxyKey = "traefik" | "coolify" | "dokploy" | "nginx" | "caddy";
const PROXIES: {
    key: ProxyKey;
    label: string;
}[] = [
    { key: "traefik", label: "Traefik" },
    { key: "coolify", label: "Coolify" },
    { key: "dokploy", label: "Dokploy" },
    { key: "nginx", label: "nginx" },
    { key: "caddy", label: "Caddy" },
];
interface Step {
    text: React.ReactNode;
    code?: string;
}
function proxySteps(key: ProxyKey, domain: string, host: string, www: boolean): Step[] {
    const names = [domain, ...(www ? [`www.${domain}`] : [])];
    const H = (h: string) => <span className="font-mono text-[11px] text-slate-800">{h}</span>;
    switch (key) {
        case "traefik":
            return [
                { text: <>Find the router rule that has {H(host)}, usually a label on the tracking container, and add the domain to it:</>, code: [host, ...names].map((n) => `Host(\`${n}\`)`).join(" || ") },
                { text: <>Apply it with <span className="font-mono text-[11px]">docker compose up -d</span>. The router&apos;s certificate resolver gets the certificates.</> },
            ];
        case "coolify":
            return [
                { text: <>Open your ColdEmail resource and the service whose Domains field has {H(host)}. Add the domain to that field:</>, code: [host, ...names].map((n) => `https://${n}`).join(",") },
                { text: "Save and redeploy. Coolify gets the certificates." },
            ];
        case "dokploy":
            return [
                { text: "Open your Cold Email compose project and go to Domains." },
                ...names.map((n) => ({ text: <>Add a domain: host {H(n)}, service tracking, port 3000, HTTPS on with Let&apos;s Encrypt.</> })),
                { text: "Deploy." },
            ];
        case "nginx":
            return [
                { text: <>In the server block that proxies {H(host)}, add the domain to its names:</>, code: `server_name ${[host, ...names].join(" ")};` },
                { text: "Keep the original hostname on the way to Cold Email, then get a certificate and reload nginx:", code: `proxy_set_header Host $host;\ncertbot --nginx ${names.map((n) => `-d ${n}`).join(" ")}` },
            ];
        case "caddy":
            return [
                { text: <>Add the domain to the site block for {H(host)}:</>, code: `${[host, ...names].join(", ")} {` },
                { text: "Reload Caddy. It gets the certificates by itself." },
            ];
    }
}
function initialProxy(proxy?: string): ProxyKey {
    return proxy === "nginx" || proxy === "caddy" ? proxy : "traefik";
}
function CodeLine({ code }: {
    code: string;
}) {
    return (<div className="mt-1 rounded-md bg-slate-900 text-slate-100 flex items-start gap-2 pl-2.5 pr-1 py-1.5">
            <pre className="min-w-0 flex-1 font-mono text-[11px] leading-relaxed whitespace-pre-wrap break-all">{code}</pre>
            <span className="[&_button]:text-slate-400 [&_button:hover]:text-white [&_button:hover]:bg-white/10">
                <CopyButton value={code} label="the snippet"/>
            </span>
        </div>);
}
/** What to change when DNS is right but visitors do not get the redirect, with Cloud as the way around it. */
