import { test, expect, fillPostEditor } from "./fixtures/test";

test("admin writes a draft, reopens it, and visitors cannot read it", async ({
  adminPage,
  request,
  adminToken,
}) => {
  const title = `E2E draft ${crypto.randomUUID()}`;
  await adminPage.goto("/admin/write");
  await fillPostEditor(adminPage, title, "Draft body 保存测试");
  const createResponse = adminPage.waitForResponse(
    (response) =>
      response.url().endsWith("/api/posts") &&
      response.request().method() === "POST",
  );
  await adminPage.getByRole("button", { name: /save draft/i }).click();
  const response = await createResponse;
  expect(response.ok(), await response.text()).toBeTruthy();
  const { post } = (await response.json()) as {
    post: { id: number; slug: string };
  };
  try {
    await expect(adminPage).toHaveURL(/\/admin\/my-posts$/);
    await adminPage.goto(`/admin/edit-post/${post.id}`);
    await expect(
      adminPage.getByRole("textbox", { name: /title/i }).first(),
    ).toHaveValue(title);
    await expect(adminPage.locator(".cm-content")).toContainText(
      "Draft body 保存测试",
    );
    const publicResponse = await request.get(`/post/${post.slug}`);
    expect(publicResponse.status()).toBe(404);
  } finally {
    await request.delete(`/api/posts/${post.id}`, {
      headers: { Authorization: `Bearer ${adminToken}` },
    });
  }
});

test("failed save leaves the editor text intact", async ({ adminPage }) => {
  await adminPage.goto("/admin/write");
  const title = `E2E failure ${crypto.randomUUID()}`;
  await fillPostEditor(adminPage, title, "Keep this text on failure");
  await adminPage.route("**/api/posts", (route) =>
    route.request().method() === "POST"
      ? route.fulfill({
          status: 500,
          body: '{"error":"test failure"}',
          contentType: "application/json",
        })
      : route.continue(),
  );
  adminPage.once("dialog", (dialog) => dialog.accept());
  await adminPage.getByRole("button", { name: /save draft/i }).click();
  await expect(
    adminPage.getByRole("textbox", { name: /title/i }).first(),
  ).toHaveValue(title);
  await expect(adminPage.locator(".cm-content")).toContainText(
    "Keep this text on failure",
  );
});
