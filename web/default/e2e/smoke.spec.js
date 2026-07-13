import { test, expect } from '@playwright/test';

const username = process.env.CCT_E2E_USERNAME || 'root';
const password = process.env.CCT_E2E_PASSWORD || '123456';

async function login(page) {
  await page.goto('/login');
  await page.locator('input[name="username"]').fill(username);
  await page.locator('input[name="password"]').fill(password);

  const loginResponse = page.waitForResponse(
    (response) =>
      response.url().includes('/api/user/login') &&
      response.request().method() === 'POST'
  );
  await page.locator('.ui.form button').first().click();

  const response = await loginResponse;
  const payload = await response.json();
  expect(payload.success, payload.message).toBe(true);
  await expect(page).not.toHaveURL(/\/login/);
}

async function expectMobileHorizontalScroll(page, selector) {
  if (page.viewportSize().width <= 768) {
    await expect(page.locator(selector)).toHaveCSS('overflow-x', 'auto');
  }
}

test.describe('Public pages', () => {
  test('homepage loads', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveTitle(/One API|CCT API/);
  });

  test('login page has form', async ({ page }) => {
    await page.goto('/login');
    await expect(page.locator('input[name="username"]')).toBeVisible();
    await expect(page.locator('input[name="password"]')).toBeVisible();
  });

  test('about page loads', async ({ page }) => {
    await page.goto('/about');
    await expect(page.locator('body')).toContainText(/About|关于/);
  });
});

test.describe('Protected pages redirect when not logged in', () => {
  test('settings redirects to login', async ({ page }) => {
    await page.goto('/setting');
    await expect(page).toHaveURL(/\/login/);
  });

  test('channel page redirects to login', async ({ page }) => {
    await page.goto('/channel');
    await expect(page).toHaveURL(/\/login/);
  });
});

test.describe('Authenticated flows', () => {
  test('login and access settings', async ({ page }) => {
    await login(page);
    await page.goto('/setting');
    await expect(page).toHaveURL(/\/setting/);
    await expect(page.locator('.settings-tab')).toBeVisible();
    await expectMobileHorizontalScroll(page, '.settings-tab');
  });

  test('free-pool page loads when authenticated', async ({ page }) => {
    await login(page);
    await page.goto('/fallback/free-pool');
    await expect(page).toHaveURL(/\/fallback\/free-pool/);
    await expect(page.locator('body')).toBeVisible();
  });
});

test.describe('Responsive admin pages', () => {
  test('channel table enables horizontal scrolling', async ({ page }) => {
    await login(page);
    await page.goto('/channel');
    await expect(page.locator('.table-responsive-wrapper')).toBeVisible();
    await expectMobileHorizontalScroll(page, '.table-responsive-wrapper');
    if (page.viewportSize().width <= 768) {
      await expect(page.locator('.table-responsive-wrapper table')).toHaveClass(
        /unstackable/
      );
    }
  });
});
