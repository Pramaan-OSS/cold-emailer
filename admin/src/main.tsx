import { StrictMode, Suspense, lazy, type ComponentType, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import "@fontsource-variable/fraunces";
import "@fontsource-variable/geist";
import "@fontsource/jetbrains-mono/400.css";
import "./global.css";
import {
    createBrowserRouter,
    Navigate,
    Outlet,
    RouterProvider,
    useLocation,
} from "react-router-dom";
import { MutationCache, QueryCache, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";


import { initErrorReporting } from "@/lib/observability";
import { reportFailure } from "@/lib/api/reportFailure";

import { Toaster } from "@/components/ui/sonner";
import { AppShell } from "@/components/layout/AppShell";
import { RequireAdmin } from "@/components/layout/RequireAdmin";
import { RouteError } from "@/components/layout/RouteError";

const LoginPage = screen(() => import("@/app/auth/LoginPage"));
const DeviceApprovalPage = screen(() => import("@/app/auth/DeviceApprovalPage"));
const OverviewPage = screen(() => import("@/app/dashboard/OverviewPage"));
const WorkersPage = screen(() => import("@/app/dashboard/WorkersPage"));
const WorkerDetailPage = screen(() => import("@/app/dashboard/WorkerDetailPage"));
const WorkerNewPage = screen(() => import("@/app/dashboard/WorkerNewPage"));
const FleetPage = screen(() => import("@/app/dashboard/FleetPage"));
const AuditPage = screen(() => import("@/app/dashboard/AuditPage"));
const OrganizationsPage = screen(() => import("@/app/dashboard/OrganizationsPage"));
const TestersPage = screen(() => import("@/app/dashboard/TestersPage"));
const OrganizationDetailPage = screen(() => import("@/app/dashboard/OrganizationDetailPage"));
const UsersPage = screen(() => import("@/app/dashboard/UsersPage"));
const UserDetailPage = screen(() => import("@/app/dashboard/UserDetailPage"));
const AdminsPage = screen(() => import("@/app/dashboard/AdminsPage"));
const WarmupPage = screen(() => import("@/app/dashboard/WarmupPage"));
const WarmupAppealsPage = screen(() => import("@/app/dashboard/WarmupAppealsPage"));
const DeveloperAppsPage = screen(() => import("@/app/dashboard/DeveloperAppsPage"));
const WarmupContentLayout = screen(() => import("@/app/dashboard/warmup-content/WarmupContentLayout"));
const WarmupContentOverviewPage = screen(() => import("@/app/dashboard/warmup-content/OverviewPage"));
const WarmupContentLibraryPage = screen(() => import("@/app/dashboard/warmup-content/LibraryPage"));
const WarmupContentJobsPage = screen(() => import("@/app/dashboard/warmup-content/JobsPage"));
const PlacementPage = screen(() => import("@/app/dashboard/PlacementPage"));
const CampaignsPage = screen(() => import("@/app/dashboard/CampaignsPage"));
const SendsPage = screen(() => import("@/app/dashboard/SendsPage"));
const LimitRequestsPage = screen(() => import("@/app/dashboard/LimitRequestsPage"));
const DiscountsPage = screen(() => import("@/app/dashboard/DiscountsPage"));
const OutreachPage = screen(() => import("@/app/dashboard/OutreachPage"));
const MailboxesPage = screen(() => import("@/app/dashboard/MailboxesPage"));
const SyncPage = screen(() => import("@/app/dashboard/SyncPage"));
const EventsPage = screen(() => import("@/app/dashboard/EventsPage"));
const JobsPage = screen(() => import("@/app/dashboard/JobsPage"));
const HealthPage = screen(() => import("@/app/dashboard/HealthPage"));
const ConfigurationPage = screen(() => import("@/app/dashboard/ConfigurationPage"));
const TransfersPage = screen(() => import("@/app/dashboard/TransfersPage"));
const NotFoundPage = screen(() => import("@/app/dashboard/NotFoundPage"));
import RealtimeManager from "@/lib/realtime/RealtimeManager";
import { RequirePermission } from "@/components/layout/RequirePermission";
import { AdminPerm } from "@/lib/auth/permissions";

// Keep each screen's code out of the initial admin download.
function screen(load: () => Promise<{ default: ComponentType }>) {
    const Component = lazy(load);
    return function Screen() {
        return <Suspense fallback={<div role="status" className="p-6 text-sm text-slate-500">Loading page…</div>}><Component /></Suspense>;
    };
}

// Mirror of web/src/main.tsx's tuned defaults. The admin app sees less
// traffic than the dashboard, so the staleness window is a touch wider:
//   - staleTime: 60s — counters and lists are fine for a minute
//   - gcTime: 5min  — keep navigation snappy on tab-back
//   - refetchOnWindowFocus: false — admin tabs sit in background all
//     day; we don't want a thundering herd of refetches on focus
//   - retry: 1 — same reasoning as the dashboard
const queryClient = new QueryClient({
    queryCache: new QueryCache({ onError: (error) => reportFailure(error, "query") }),
    mutationCache: new MutationCache({ onError: (error) => reportFailure(error, "mutation") }),
    defaultOptions: {
        queries: {
            staleTime: 60_000,
            gcTime: 5 * 60_000,
            refetchOnWindowFocus: false,
            refetchOnReconnect: "always",
            retry: 1,
        },
        mutations: { retry: 0 },
    },
});

// Every gated route carries the same permission bit the backend gates its
// endpoint on, with the human name RequirePermission shows on denial.
const PERM_LABELS: Record<number, string> = {
    [AdminPerm.ViewUsers]: "View users",
    [AdminPerm.ViewWorkers]: "View workers",
    [AdminPerm.ViewWarmupPool]: "View warmup pool",
    [AdminPerm.ReviewAppeals]: "Review appeals",
    [AdminPerm.ViewCampaigns]: "View campaigns",
    [AdminPerm.ViewAnalytics]: "View analytics",
    [AdminPerm.ViewAuditLogs]: "View audit logs",
    [AdminPerm.ManageSettings]: "Manage settings",
    [AdminPerm.GrantAdminAccess]: "Grant admin access",
    [AdminPerm.ViewOrganizations]: "View organizations",
};

function gated(perm: number, page: ReactNode) {
    return (
        <RequirePermission perm={perm} permissionLabel={PERM_LABELS[perm] ?? "required"}>
            {page}
        </RequirePermission>
    );
}

// Old paths that docs link. The query string travels so `?tab=` survives
// and a moved page can pin the tab it replaced.
function Redirect({ to, tab }: { to: string; tab?: string }) {
    const location = useLocation();
    const params = new URLSearchParams(location.search);
    if (tab && !params.has("tab")) params.set("tab", tab);
    const search = params.toString();
    return <Navigate to={search ? `${to}?${search}` : to} replace />;
}

const router = createBrowserRouter([
    {
        path: "/auth/login",
        element: <LoginPage />,
    },
    {
        path: "/",
        element: <RequireAdmin />,
        children: [
            {
                element: <AppShellWithKey />,
                errorElement: <RouteError />,
                children: [
                    {
                        // Pathless: a page that throws renders RouteError inside
                        // the shell instead of replacing the whole app.
                        errorElement: <RouteError />,
                        children: [
                            { index: true, element: <OverviewPage /> },
                            { path: "device", element: <DeviceApprovalPage /> },

                            // Operations
                            { path: "workers", element: gated(AdminPerm.ViewWorkers, <WorkersPage />) },
                            // Before :id so "new" isn't parsed as a worker id.
                            { path: "workers/new", element: gated(AdminPerm.ViewWorkers, <WorkerNewPage />) },
                            { path: "workers/:id", element: gated(AdminPerm.ViewWorkers, <WorkerDetailPage />) },
                            { path: "fleet", element: gated(AdminPerm.ViewWorkers, <FleetPage />) },
                            { path: "mailboxes", element: gated(AdminPerm.ViewUsers, <MailboxesPage />) },
                            { path: "sync", element: gated(AdminPerm.ViewUsers, <SyncPage />) },
                            { path: "warmup", element: gated(AdminPerm.ViewWarmupPool, <WarmupPage />) },
                            { path: "warmup/appeals", element: gated(AdminPerm.ReviewAppeals, <WarmupAppealsPage />) },
                            {
                                path: "warmup-content",
                                element: gated(AdminPerm.ViewWarmupPool, <WarmupContentLayout />),
                                children: [
                                    { index: true, element: <Navigate to="/warmup-content/overview" replace /> },
                                    { path: "overview", element: <WarmupContentOverviewPage /> },
                                    { path: "library", element: <WarmupContentLibraryPage /> },
                                    { path: "jobs", element: <WarmupContentJobsPage /> },
                                ],
                            },
                            { path: "placement", element: gated(AdminPerm.ViewWarmupPool, <PlacementPage />) },
                            { path: "campaigns", element: gated(AdminPerm.ViewCampaigns, <CampaignsPage />) },
                            { path: "sends", element: gated(AdminPerm.ViewCampaigns, <SendsPage />) },

                            // Accounts
                            { path: "users", element: gated(AdminPerm.ViewUsers, <UsersPage />) },
                            { path: "users/:id", element: gated(AdminPerm.ViewUsers, <UserDetailPage />) },
                            { path: "organizations", element: gated(AdminPerm.ViewOrganizations, <OrganizationsPage />) },
                            { path: "organizations/:id", element: gated(AdminPerm.ViewOrganizations, <OrganizationDetailPage />) },
                            { path: "limit-requests", element: gated(AdminPerm.ViewOrganizations, <LimitRequestsPage />) },
                            { path: "developer-apps", element: gated(AdminPerm.ViewOrganizations, <DeveloperAppsPage />) },
                            { path: "app-listings", element: <Navigate to="/developer-apps?tab=directory" replace /> },
                            { path: "discounts", element: gated(AdminPerm.ViewOrganizations, <DiscountsPage />) },
                            { path: "outreach", element: gated(AdminPerm.ViewOrganizations, <OutreachPage />) },
                            { path: "admins", element: gated(AdminPerm.GrantAdminAccess, <AdminsPage />) },
                            { path: "testers", element: gated(AdminPerm.ViewUsers, <TestersPage />) },

                            // Insight
                            { path: "events", element: <EventsPage /> },
                            { path: "audit", element: gated(AdminPerm.ViewAuditLogs, <AuditPage />) },
                            { path: "jobs", element: gated(AdminPerm.ViewAnalytics, <JobsPage />) },

                            // Instance
                            { path: "health", element: gated(AdminPerm.ViewAnalytics, <HealthPage />) },
                            { path: "configuration", element: gated(AdminPerm.ManageSettings, <ConfigurationPage />) },
                            { path: "transfers", element: gated(AdminPerm.ViewOrganizations, <TransfersPage />) },

                            // Retired paths that docs still link.
                            { path: "analytics", element: <Redirect to="/" /> },
                            { path: "system", element: <Redirect to="/health" tab="services" /> },
                            { path: "configuration/settings", element: <Redirect to="/configuration" tab="settings" /> },
                            { path: "configuration/notifications", element: <Redirect to="/configuration" tab="notifications" /> },
                            { path: "limits", element: <Redirect to="/configuration" tab="limits" /> },

                            { path: "*", element: <NotFoundPage /> },
                        ],
                    },
                ],
            },
        ],
    },
]);

// AppShell renders an <Outlet/> for the page. RealtimeManager is mounted
// here (inside QueryClientProvider, only for authenticated admins) so the
// admin:platform socket connects once and survives route changes.
function AppShellWithKey() {
    return (
        <>
            <RealtimeManager />
            <AppShell />
        </>
    );
}

// Tiny outlet helper exported so React-Router's typing is happy when
// we need a passthrough.
export { Outlet };

// Before the first render, so a boot failure is reported too.
initErrorReporting();

createRoot(document.getElementById("root")!).render(
    <StrictMode>
        <QueryClientProvider client={queryClient}>
            <RouterProvider router={router} />
            <Toaster position="top-center" />
            {import.meta.env.DEV && <ReactQueryDevtools initialIsOpen={false} />}
        </QueryClientProvider>
    </StrictMode>,
);
