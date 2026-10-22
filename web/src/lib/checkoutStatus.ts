import Request from "@/lib/api/client/Request";

const storageKey = (workspace: string) => `checkout:${workspace}`;
export function rememberCheckout(workspace: string, ref: string) {
    if (!workspace || !ref) throw new Error("Checkout needs review before opening another payment page.");
    sessionStorage.setItem(storageKey(workspace), ref);
}
export async function checkoutStatus(workspace: string): Promise<string> {
    const ref = sessionStorage.getItem(storageKey(workspace));
    if (!ref) return "Payment has not been confirmed. Check your billing status before paying again.";
    const result = await Request<{ status: string; confirmed: boolean }>({
        method: "GET", url: "/subscription/checkout", params: { ref }, authorization: true,
    });
    if (result.confirmed === true && result.status === "paid") return "Payment confirmed. Your plan will update after billing finishes processing.";
    if (result.status === "unpaid" || result.status === "pending" || result.status === "active") return "Payment is still pending. Check its status before paying again.";
    return "Payment has not been confirmed. Check your billing status before paying again.";
}
