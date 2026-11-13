import { test, expect, fillPostEditor } from "./fixtures/test";

test("published post is visible to a visitor and updates through the editor", async ({
  adminPage,
  page,
  createPost,
}) => {
  const post = await createPost({
    status: "published",
    content: "Original public body",
  });
  await page.goto(`/post/${post.slug}`);
  await expect(page.getByRole("heading", { name: post.title })).toBeVisible();
  await expect(page.getByText("Original public body")).toBeVisible();

  await adminPage.goto(`/admin/edit-post/${post.id}`);
  const updated = `Updated E2E ${crypto.randomUUID()}`;
  await adminPage
    .getByRole("textbox", { name: /title/i })
    .first()
    .fill(updated);
  await adminPage.locator(".cm-content").fill("Updated public body");
  const save = adminPage.waitForResponse(
    (response) =>
      response.url().endsWith(`/api/posts/${post.id}`) &&
      response.request().method() === "PUT",
  );
  await adminPage.getByRole("button", { name: /update/i }).click();
  expect((await save).ok()).toBeTruthy();
  await page.reload();
  await expect(page.getByRole("heading", { name: updated })).toBeVisible();
  await expect(page.getByText("Updated public body")).toBeVisible();
});

test("admin publishes a new article from the UI", async ({
  adminPage,
  page,
  request,
  adminToken,
}) => {
  const title = `E2E publish ${crypto.randomUUID()}`;
  await adminPage.goto("/admin/write");
  await fillPostEditor(adminPage, title, "Public body from the UI");
  const create = adminPage.waitForResponse(
    (response) =>
      response.url().endsWith("/api/posts") &&
      response.request().method() === "POST",
  );
  await adminPage.getByRole("button", { name: /^publish$/i }).click();
  const response = await create;
  expect(response.ok(), await response.text()).toBeTruthy();
  const { post } = (await response.json()) as {
    post: { id: number; slug: string };
  };
  try {
    await page.goto(`/post/${post.slug}`);
    await expect(page.getByRole("heading", { name: title })).toBeVisible();
  } finally {
    await request.delete(`/api/posts/${post.id}`, {
      headers: { Authorization: `Bearer ${adminToken}` },
    });
  }
});
