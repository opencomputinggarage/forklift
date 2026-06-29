import { expect, test } from "@playwright/test";

test("renders the login screen", async ({ page }) => {
  await page.goto("/login");

  await expect(page.getByText("forklift")).toBeVisible();
  await expect(page.getByLabel(/username|사용자 이름/i)).toBeVisible();
  await expect(page.getByLabel(/password|비밀번호/i)).toBeVisible();
});
