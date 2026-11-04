import { test, expect } from "./fixtures/test";

test("visitor is redirected away from the protected console", async ({
  page,
}) => {
  await page.goto("/admin/users");
  await expect(page).toHaveURL(/\/admin\/login$/);
});

test("unauthenticated API requests cannot list users or create posts", async ({
  request,
}) => {
  const users = await request.get("/api/users");
  expect(users.status()).toBe(401);
  const create = await request.post("/api/posts", {
    data: {
      slug: "not-allowed",
      title: "Not allowed",
      content: "text",
      category: "Default",
    },
  });
  expect(create.status()).toBe(401);
});

test("admin can access user management", async ({ adminPage }) => {
  await adminPage.goto("/admin/users");
  await expect(adminPage).toHaveURL(/\/admin\/users$/);
  await expect(adminPage.getByRole("heading").first()).toBeVisible();
});
