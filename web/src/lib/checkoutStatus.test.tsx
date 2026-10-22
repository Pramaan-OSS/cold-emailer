import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, waitFor, cleanup } from "@testing-library/react";
import Request from "@/lib/api/client/Request";
import { checkoutStatus, rememberCheckout } from "./checkoutStatus";
import CheckoutStatus from "@/components/app/billing/CheckoutStatus";
vi.mock("@/lib/api/client/Request", () => ({ default: vi.fn() }));
beforeEach(() => { cleanup(); vi.clearAllMocks(); sessionStorage.clear(); window.history.replaceState({}, "", "/?checkout=success"); });
it("reads a saved checkout reference without creating another", async () => {
 rememberCheckout("one", "checkout_reference");
 vi.mocked(Request).mockResolvedValue({ status: "paid", confirmed: true });
 render(<CheckoutStatus workspace="one" />);
 await screen.findByText(/Payment confirmed/);
 expect(Request).toHaveBeenCalledOnce();
 expect(Request).toHaveBeenCalledWith(expect.objectContaining({ method: "GET", params: { ref: "checkout_reference" } }));
});
it.each(["unknown", "completed", "pending", "unpaid"])("never calls %s a confirmed payment", async status => {
 rememberCheckout("one", "checkout_reference"); vi.mocked(Request).mockResolvedValue({ status, confirmed: false });
 expect(await checkoutStatus("one")).not.toMatch(/Payment confirmed/);
 expect(Request).toHaveBeenCalledOnce();
});
it("does not trust the return URL or use another workspace reference", async () => {
 rememberCheckout("one", "checkout_reference");
 expect(await checkoutStatus("two")).toMatch(/not been confirmed/);
 expect(Request).not.toHaveBeenCalled();
});
it("shows unavailable without retrying or hiding the rest of the page", async () => {
 rememberCheckout("one", "checkout_reference");vi.mocked(Request).mockRejectedValue(new Error("This feature is not available yet."));
 render(<><CheckoutStatus workspace="one" /><p>Billing settings</p></>);
 await waitFor(() => expect(screen.getByRole("status").textContent).toMatch(/Check billing status before paying again/));
 expect(screen.getByText("Billing settings")).toBeTruthy();expect(Request).toHaveBeenCalledOnce();
});
it("refuses a missing reference before redirect", () => { expect(() => rememberCheckout("one", "")).toThrow(/needs review/); });
