import type { Page } from "@playwright/test";
import { env } from "../lib/env.ts";
import { expect, test } from "../lib/proof.ts";

// The Advisor evaluates in the background; ask for a run so a fresh seed has findings to suggest.
async function evaluateAdvisor(page: Page) {
  await page.evaluate(async (api) => {
    const token = JSON.parse(localStorage.getItem("auth_token") ?? "{}").access_token;
    await fetch(`${api}/v1/advisor/refresh`, { method: "POST", headers: { Authorization: `Bearer ${token}` } });
  }, env.apiURL);
}

test("Pramaan opens as a floating window and suggests fixes for what the Advisor found", async ({ page, proof }) => {
  await page.goto("/app/emails");
  await expect(page.getByRole("row", { name: /dev\.outbound@coldemail\.test/ })).toBeVisible();
  await evaluateAdvisor(page);
  await page.reload();
  await expect(page.getByRole("row", { name: /dev\.outbound@coldemail\.test/ })).toBeVisible();
  await proof.chapter("Pramaan", "The assistant opens as a floating window with fixes for what the Advisor measured");

  await page.getByRole("button", { name: "Ask Pramaan" }).click();
  await expect(page.getByText("Hi, I'm Pramaan")).toBeVisible();
  await expect(page.getByText("Suggested for you")).toBeVisible();
  await proof.dwell(1200);
  await proof.shot("pramaan-suggestions", { caption: "Pramaan floats over the page and suggests fixes found in this workspace" });
});

test("A Pramaan run shows live status, folded tool steps, an approval and the answer", async ({ page, proof }) => {
  await page.goto("/app/emails");
  await expect(page.getByRole("row", { name: /dev\.outbound@coldemail\.test/ })).toBeVisible();
  await page.getByRole("button", { name: "Ask Pramaan" }).click();
  await expect(page.getByText("Hi, I'm Pramaan")).toBeVisible();
  await proof.chapter("A Pramaan run", "A scripted run (dev builds only) through the same rendering a live run uses");

  const composer = page.getByPlaceholder(/Ask Pramaan about/);
  await composer.pressSequentially("/demo", { delay: 60 });
  await composer.press("Enter");
  await expect(page.getByText("Reading campaign stats").first()).toBeVisible();
  await proof.shot("pramaan-working", {
    caption: "While Pramaan works: a live status with a timer, and each tool step as it runs",
    ignore: [page.getByText(/^\d+\.\ds$/)],
  });

  const approve = page.getByRole("button", { name: "Approve", exact: true });
  await expect(approve).toBeVisible({ timeout: 20_000 });
  await proof.dwell(1500);
  await proof.shot("pramaan-approval", { caption: "Steps fold away, the answer streams in, and a change waits for approval" });

  await approve.click();
  await expect(page.getByText(/is paused and nothing else was changed/)).toBeVisible({ timeout: 15_000 });
  await proof.dwell(1200);
  await proof.shot("pramaan-done", { caption: "Approved: the change runs and Pramaan reports back" });
});
