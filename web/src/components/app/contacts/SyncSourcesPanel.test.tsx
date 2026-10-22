import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import SyncSourcesPanel from "./SyncSourcesPanel";
afterEach(cleanup);
it("sends spreadsheet discovery to Connections without starting consent or a paid search", () => {
 const fetch = vi.spyOn(globalThis, "fetch");
 const close = vi.fn();render(<SyncSourcesPanel open onClose={close} />);
 expect(screen.getByRole("link", { name: "Find spreadsheet actions" }).getAttribute("href")).toBe("/app/integrations?query=spreadsheet");
 expect(screen.getByText(/Scheduled spreadsheet syncing is not available yet/)).toBeTruthy();
 expect(fetch).not.toHaveBeenCalled();fireEvent.keyDown(window, { key: "Escape" });expect(close).toHaveBeenCalledOnce();fetch.mockRestore();
});
