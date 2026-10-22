import React from "react";
import { XIcon } from "lucide-react";

// Spreadsheet discovery and account consent belong to the capability-backed Connections screen.
export default function SyncSourcesPanel({ open, onClose }: {
    open: boolean;
    onClose: () => void;
    campaign?: { id: string; name: string };
    segment?: { id: string; name: string; color?: string };
}) {
    React.useEffect(() => {
        if (!open) return;
        const escape = (event: KeyboardEvent) => { if (event.key === "Escape") onClose(); };
        window.addEventListener("keydown", escape);
        return () => window.removeEventListener("keydown", escape);
    }, [open, onClose]);
    if (!open) return null;
    return <div className="fixed inset-0 z-[110] flex items-center justify-center bg-slate-900/30 p-4" onMouseDown={event => { if (event.target === event.currentTarget) onClose(); }}>
        <section role="dialog" aria-modal="true" aria-label="Spreadsheet connections" className="w-full max-w-md rounded-md border border-slate-200 bg-white p-5 shadow-lg space-y-4">
            <div className="flex items-center justify-between"><h2 className="text-sm font-semibold">Spreadsheet connections</h2><button type="button" aria-label="Close" onClick={onClose}><XIcon className="size-4" /></button></div>
            <p className="text-sm text-slate-600">Find spreadsheet actions in Connections, connect your account and review the fields returned by the catalogue before running an action.</p>
            <p className="text-sm text-slate-600">Scheduled spreadsheet syncing is not available yet. You can import a spreadsheet exported as CSV from Contacts.</p>
            <a href="/app/integrations?query=spreadsheet" className="inline-flex rounded-md bg-slate-900 px-3 py-2 text-sm text-white">Find spreadsheet actions</a>
        </section>
    </div>;
}
