import { test, expect } from "./fixtures/test";

test("editor uploads an image and inserts it into the post body", async ({
  adminPage,
  request,
  adminToken,
}) => {
  await adminPage.goto("/admin/write");
  await adminPage.getByRole("button", { name: /image/i }).first().click();
  const dialog = adminPage.getByRole("dialog");
  await expect(dialog).toBeVisible();
  const upload = adminPage.waitForResponse(
    (response) =>
      response.url().endsWith("/api/assets/upload") &&
      response.request().method() === "POST",
  );
  await dialog.locator('input[type="file"]').setInputFiles({
    name: "e2e.png",
    mimeType: "image/png",
    buffer: Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=",
      "base64",
    ),
  });
  const response = await upload;
  expect(response.ok()).toBeTruthy();
  const asset = (await response.json()) as { assetPublicID: string };
  try {
    await dialog.getByRole("button", { name: /insert|save|confirm/i }).click();
    await expect(
      adminPage.locator(".cm-content").getByRole("img", { name: "e2e" }),
    ).toBeVisible();
  } finally {
    await request.post(`/api/assets/delete/${asset.assetPublicID}`, {
      headers: { Authorization: `Bearer ${adminToken}` },
    });
  }
});

test("unsupported upload is rejected without leaving the editor", async ({
  adminPage,
}) => {
  await adminPage.goto("/admin/write");
  await adminPage.getByRole("button", { name: /image/i }).first().click();
  const dialog = adminPage.getByRole("dialog");
  const upload = adminPage.waitForResponse(
    (response) =>
      response.url().endsWith("/api/assets/upload") &&
      response.request().method() === "POST",
  );
  await dialog.locator('input[type="file"]').setInputFiles({
    name: "e2e.txt",
    mimeType: "text/plain",
    buffer: Buffer.from("not an image"),
  });
  expect((await upload).ok()).toBe(false);
  await expect(adminPage).toHaveURL(/\/admin\/write$/);
  await expect(dialog).toBeVisible();
});
