import React from "react";
import { checkoutStatus } from "@/lib/checkoutStatus";
import buildError from "@/lib/helper/buildError";

export default function CheckoutStatus({ workspace }: { workspace: string }) {
    const [message, setMessage] = React.useState("");
    React.useEffect(() => {
        let active = true;
        if (!workspace || !["success", "done"].includes(new URLSearchParams(window.location.search).get("checkout") ?? "")) return;
        setMessage("Checking payment status…");
        checkoutStatus(workspace).then(value => { if (active) setMessage(value); }).catch(error => {
            if (active) setMessage(`${buildError(error)} Check billing status before paying again.`);
        });
        return () => { active = false; };
    }, [workspace]);
    return message ? <p role="status" className="px-6 py-3 text-sm text-slate-700">{message}</p> : null;
}
