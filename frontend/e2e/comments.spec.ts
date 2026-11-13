import { test, expect } from "./fixtures/test";

test("admin moderates a pending comment and visitor sees the result", async ({
  adminPage,
  request,
  adminToken,
  createPost,
}) => {
  const post = await createPost({ status: "published" });
  const headers = { Authorization: `Bearer ${adminToken}` };
  const configResponse = await request.get("/api/moderation/comments/config", {
    headers,
  });
  expect(configResponse.ok()).toBeTruthy();
  const config = (await configResponse.json()) as {
    manualReviewEnabled: boolean;
    keywordFilterEnabled: boolean;
    llmReviewEnabled: boolean;
  };
  await request.put("/api/moderation/comments/config", {
    headers,
    data: {
      manualReviewEnabled: true,
      keywordFilterEnabled: false,
      llmReviewEnabled: false,
    },
  });
  let commentId: number | undefined;
  try {
    const content = `E2E pending comment ${crypto.randomUUID()}`;
    const created = await request.post("/api/comments", {
      headers,
      data: { postId: post.id, content },
    });
    expect(created.ok(), await created.text()).toBeTruthy();
    const body = (await created.json()) as {
      comment: { id: number };
      requiresModeration: boolean;
    };
    commentId = body.comment.id;
    expect(body.requiresModeration).toBe(true);
    await adminPage.goto("/admin/comment-moderation");
    const row = adminPage.locator("li").filter({ hasText: content });
    await expect(row).toBeVisible();
    await row.getByRole("button", { name: /approve/i }).click();
    const visible = await request.get(`/api/comments/post/${post.id}`);
    expect(await visible.text()).toContain(content);
  } finally {
    if (commentId)
      await request.delete(`/api/comments/${commentId}`, { headers });
    await request.put("/api/moderation/comments/config", {
      headers,
      data: config,
    });
  }
});
