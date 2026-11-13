import { test, expect, loginThroughUI } from "./fixtures/test";

test("admin logs in and remains signed in after reload", async ({ page }) => {
  await loginThroughUI(page);
  await expect(page).toHaveURL(/\/admin$/);
  await expect(
    page.getByRole("heading", { name: /admin dashboard/i }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole("heading", { name: /admin dashboard/i }),
  ).toBeVisible();
});

test("wrong password keeps the visitor on the login page", async ({ page }) => {
  await loginThroughUI(page, "incorrect-e2e-password");
  await expect(page).toHaveURL(/\/admin\/login$/);
  await expect(page.getByRole("alert")).toBeVisible();
});

test("empty credentials cannot be submitted", async ({ page }) => {
  await page.goto("/admin/login");
  await page.getByRole("button", { name: /^log in$|^login$/i }).click();
  await expect(page).toHaveURL(/\/admin\/login$/);
  await expect(page.locator("#email")).toHaveAttribute("required", "");
});
