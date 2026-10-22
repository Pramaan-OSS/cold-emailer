import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { OrgSwitcher } from "./OrgSwitcher";
const state = vi.hoisted(() => ({ logout: vi.fn(), navigate: vi.fn() }));
vi.mock("@tanstack/react-router", () => ({ useNavigate: () => state.navigate }));
vi.mock("@/lib/api/hooks/auth/useLogout", () => ({ default: () => ({ mutateAsync: state.logout, isPending: false }) }));
vi.mock("@/lib/api/hooks/app/organizations/useSwitchOrganization", () => ({ default: () => ({ isPending: false }) }));
vi.mock("@/components/app/organizations/NewWorkspaceDialog", () => ({ NewWorkspaceDialog: () => null }));
vi.mock("@/stores", () => ({ useAppStore: (fn: (s: unknown) => unknown) => fn({ organizations: [{ id: "one", name: "QA workspace" }], currentOrganization: { id: "one", name: "QA workspace" } }) }));
it("offers sign-out and waits for session cleanup before leaving", async () => {
 let finish!: () => void;
 state.logout.mockReturnValue(new Promise<void>(resolve => { finish = resolve; }));
 render(<OrgSwitcher />);
 fireEvent.click(screen.getByRole("button", { name: /QA workspace/ }));
 fireEvent.click(await screen.findByRole("menuitem", { name: "Sign out" }));
 expect(state.logout).toHaveBeenCalledOnce();expect(state.navigate).not.toHaveBeenCalled();
 finish();await waitFor(() => expect(state.navigate).toHaveBeenCalledWith({ to: "/auth/login", replace: true }));
});
