import PramaanSetupNotice from '@/components/shared/PramaanSetupNotice';
// The new app shell.
//
// Layout:
//   ┌──────────────────────────────────────────────────────┐
//   │  [logo]  >  [org]  >  [section]               [⌘K ●] │  AppHeader
//   ├──────────┬───────────────────────────────────────────┤
//   │          │ ╭─── content ──────────────────────────╮  │
//   │  AppNav  │ │                                      │  │
//   │          │ │                                      │  │
//   │          │ │                                      │  │
//   └──────────┴───────────────────────────────────────────┘
//
// The header + sidebar share one sky-coloured chrome layer (SkyChrome).
// The content panel sits in the bottom-right with a rounded top-left
// where it meets the chrome's inner corner. Reads as one continuous
// frame around a clean work surface.

import { Suspense, useRef, useState } from "react";
import { Outlet, useLocation } from "@tanstack/react-router";
import SubscriptionGate from "./SubscriptionGate";

import { Link } from "@tanstack/react-router";
import { OrgSwitcher } from "./OrgSwitcher";
import { Logo } from "@/components/svg";

import PendingDeletionBar from "./PendingDeletionBar";
import SendingRestrictedBar from "./SendingRestrictedBar";
import { RouteBoundary } from "./ErrorBoundary";
import { ShortcutsModal } from "@/components/shared/ShortcutsModal";
import { CommandPalette } from "@/components/shared/CommandPalette";
import { useKeyboardShortcuts } from "@/hooks/useKeyboardShortcuts";
import { GlobalCursorsProvider } from "@/components/app/presence/GlobalCursors";
import AgentPanel from "@/components/app/agent/AgentPanel";

import { useRouteKey } from "@/hooks/useRouteKey";
import { useScrollMemory, type ScrollStore } from "@/hooks/useScrollMemory";
import { RouteFallback } from "./RouteStates";

// Per history entry, kept apart so ordinary browsing never evicts a list's remembered offset.
const entryOffsets: ScrollStore = new Map();

export function AppShell() {
    useKeyboardShortcuts();

    // Mobile nav drawer. On >=md the sidebar is a static column and this is
    // ignored; below md it's an off-canvas drawer toggled from the header.

    const { pathname } = useLocation();
    // Close the drawer whenever the route changes (tapping a nav link).


    // The page content's scroll container, anchor for the global cursor layer.
    const scrollRef = useRef<HTMLDivElement>(null);

    // A new page starts at the top and Back lands where that entry was left; in-page URL state keeps its place.
    const routeKey = useRouteKey();
    const entryKey = useLocation({ select: (l) => l.state.__TSR_key ?? l.href });
    const [scrollFor, setScrollFor] = useState({ routeKey, key: `${routeKey}@${entryKey}` });
    if (scrollFor.routeKey !== routeKey) setScrollFor({ routeKey, key: `${routeKey}@${entryKey}` });
    useScrollMemory(scrollRef, scrollFor.key, entryOffsets);

    const pages = [["Inbox", "/app/unibox/{-$scope}/{-$threadId}"], ["Campaigns", "/app/campaigns"], ["Contacts", "/app/contacts"], ["Connections", "/app/integrations"], ["Settings", "/app/settings"]] as const;
    return (
        <div className="ce-shell">
            <header className="top">
                <Link to="/app/unibox/{-$scope}/{-$threadId}" className="brand"><Logo className="size-7" />Cold Email</Link>
                <nav className="crumb" aria-label="Breadcrumb"><Link to="/app/unibox/{-$scope}/{-$threadId}">Workspace</Link><span className="sep" aria-hidden>/</span><OrgSwitcher /></nav>
                <Link to="/app/settings" className="btn">Settings</Link>
            </header>
            <PramaanSetupNotice /><SendingRestrictedBar /><PendingDeletionBar />
            <main className="ce-main" data-thread-none>
                <aside className="left">
                    <div className="strip">Conversations and changes stay in this workspace.</div>
                    <AgentPanel embedded />
                </aside>
                <section className="right">
                    <nav className="tabs" aria-label="Workspace">
                        {pages.map(([label, to]) => <Link key={to} to={to} className="tab" aria-current={pathname.startsWith(to.split("/{")[0]) ? "page" : undefined}>{label}</Link>)}
                    </nav>
                    <GlobalCursorsProvider scrollRef={scrollRef}>
                        <div ref={scrollRef} className="view">
                            <RouteBoundary><Suspense fallback={<RouteFallback />}><SubscriptionGate><Outlet /></SubscriptionGate></Suspense></RouteBoundary>
                        </div>
                    </GlobalCursorsProvider>
                </section>
            </main>
            <ShortcutsModal /><CommandPalette />
        </div>
    );
}
