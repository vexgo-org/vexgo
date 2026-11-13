import { test as base, expect, type Page } from "@playwright/test";

const adminEmail = "admin@example.com";
const adminPassword = "password";

type Post = { id: number; slug: string; title: string; status: string };
type PostOptions = {
  title?: string;
  content?: string;
  status?: "draft" | "published";
};
type Fixtures = {
  adminToken: string;
  adminPage: Page;
  createPost: (options?: PostOptions) => Promise<Post>;
};

export const test = base.extend<Fixtures>({
  adminToken: async ({ request }, provide) => {
    const response = await request.post("/api/auth/login", {
      data: { email: adminEmail, password: adminPassword },
    });
    expect(response.ok()).toBeTruthy();
    const body = (await response.json()) as { token: string };
    await provide(body.token);
  },
  adminPage: async ({ browser, adminToken, baseURL }, provide) => {
    const context = await browser.newContext({ baseURL });
    await context.addInitScript((token) => {
      localStorage.setItem("token", token);
      localStorage.setItem(
        "user",
        JSON.stringify({
          username: "admin",
          email: "admin@example.com",
          role: "super_admin",
        }),
      );
    }, adminToken);
    const page = await context.newPage();
    await provide(page);
    await context.close();
  },
  createPost: async ({ request, adminToken }, provide) => {
    const created: number[] = [];
    const create = async (options: PostOptions = {}) => {
      const suffix = crypto.randomUUID();
      const response = await request.post("/api/posts", {
        headers: { Authorization: `Bearer ${adminToken}` },
        data: {
          slug: `e2e-${suffix}`,
          title: options.title ?? `E2E post ${suffix}`,
          content: options.content ?? "E2E post body",
          category: "Default",
          tags: [],
          status: options.status ?? "draft",
        },
      });
      expect(response.ok(), await response.text()).toBeTruthy();
      const body = (await response.json()) as { post: Post };
      created.push(body.post.id);
      return body.post;
    };
    await provide(create);
    for (const id of created) {
      await request.delete(`/api/posts/${id}`, {
        headers: { Authorization: `Bearer ${adminToken}` },
      });
    }
  },
});

export { expect };

export async function loginThroughUI(page: Page, password = adminPassword) {
  await page.goto("/admin/login");
  await page.locator("#email").fill(adminEmail);
  await page.locator("#password").fill(password);
  await page.getByRole("button", { name: /^log in$|^login$/i }).click();
}

export async function fillPostEditor(
  page: Page,
  title: string,
  content: string,
) {
  await page.getByRole("textbox", { name: /title/i }).first().fill(title);
  await page.locator("#slug").fill(`e2e-${crypto.randomUUID()}`);
  await page.locator(".cm-content").fill(content);
}
