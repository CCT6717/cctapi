import { test, expect } from '@playwright/test';

const username = process.env.CCT_E2E_USERNAME || 'root';
const password = process.env.CCT_E2E_PASSWORD || '123456';

test.use({
  baseURL: process.env.CCT_E2E_BASE_URL || 'http://127.0.0.1:3008',
});

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

  test('fallback status diagnostics fit the mobile viewport', async ({ page }) => {
    let attemptObservabilityRequestCount = 0;
    await login(page);
    await page.route('**/api/fallback/virtual-models', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: [{ name: 'free/auto', strategy: 'free_first', enabled: true, pools: ['free'] }],
        }),
      }),
    );
    await page.route('**/api/fallback/deployments/runtime-status', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: [{
            deployment_id: 'free:active',
            pool: 'free',
            health: 'rate_limited',
            provider_rate_limit_degradation: {
              active: true,
              level: 2,
              episode_count: 3,
              next_recovery_at: '2026-07-16T08:30:00Z',
            },
          }],
        }),
      }),
    );
    await page.route('**/api/fallback/attempt-observability', (route) => {
      attemptObservabilityRequestCount += 1;
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            generated_at: '2026-07-17T03:00:02Z',
            failure_window_seconds: 3600,
            failure_event_count: 1,
            skip_event_count: 0,
            top_deployments: [{
              key: 'free:kilo-e2e',
              deployment_id: 'free:kilo-e2e',
              count: 1,
              last_seen_at: '2026-07-17T03:00:00Z',
            }],
            top_providers: [{
              key: 'kilo',
              provider: 'kilo',
              count: 1,
              last_seen_at: '2026-07-17T03:00:00Z',
            }],
            top_models: [{
              key: 'kilo/e2e-primary:free',
              real_model: 'kilo/e2e-primary:free',
              count: 1,
              last_seen_at: '2026-07-17T03:00:00Z',
            }],
            error_categories: [{
              key: 'rate_limit',
              category: 'rate_limit',
              count: 1,
              last_seen_at: '2026-07-17T03:00:00Z',
            }],
            outcomes: [{
              key: 'model_rate_limited',
              outcome: 'model_rate_limited',
              count: 1,
              last_seen_at: '2026-07-17T03:00:00Z',
            }],
            recent_chains: [{
              request_id: 'e2e-attempt-request-429',
              virtual_model: 'openrouter/auto',
              started_at: '2026-07-17T03:00:00Z',
              finished_at: '2026-07-17T03:00:01Z',
              steps: [
                {
                  created_at: '2026-07-17T03:00:00Z',
                  provider: 'kilo',
                  deployment_id: 'free:kilo-e2e',
                  real_model: 'kilo/e2e-primary:free',
                  outcome: 'model_rate_limited',
                  status_code: 429,
                  error_category: 'rate_limit',
                  duration_ms: 320,
                  stream_written: false,
                  plan_index: 0,
                  upstream_attempt_index: 1,
                },
                {
                  created_at: '2026-07-17T03:00:01Z',
                  provider: 'kilo',
                  deployment_id: 'free:kilo-e2e',
                  real_model: 'kilo/e2e-recovery:free',
                  outcome: 'success',
                  status_code: 200,
                  error_category: 'none',
                  duration_ms: 410,
                  stream_written: false,
                  plan_index: 0,
                  upstream_attempt_index: 2,
                },
              ],
            }],
          },
        }),
      });
    });

    await page.goto('/fallback/status');
    await page.locator('.gw-row-top').click();
    const diagnostic = page.locator('.gw-degradation');
    await expect(diagnostic).toBeVisible();
    await expect(diagnostic).toContainText('持续限流降权 L2');
    if (page.viewportSize().width <= 768) {
      await expect(diagnostic).toHaveCSS('flex-direction', 'column');
    }

    await page.getByRole('link', { name: /使用统计/ }).click();
    await expect(page.getByRole('heading', { name: '运行数据' })).toBeVisible();
    const attemptDiagnostic = page.locator('.fallback-attempt-section');
    await expect(
      attemptDiagnostic.getByRole('heading', { name: '精准失败诊断' })
    ).toBeVisible();
    await expect(
      attemptDiagnostic.getByRole('heading', { name: '最近请求链路' })
    ).toBeVisible();
    expect(attemptObservabilityRequestCount).toBeGreaterThan(0);
    const attemptChain = page.locator('.fallback-attempt-chain-card').filter({
      hasText: 'e2e-attempt-request-429',
    });
    await expect(attemptChain).toHaveCount(1);
    await expect(attemptChain).toContainText('e2e-attempt-request-429');
    await expect(attemptChain).toContainText('kilo/e2e-primary:free');
    await expect(attemptChain).toContainText('HTTP 429');
    await expect(attemptChain).toContainText('限速');
    await expect(attemptChain).toContainText('kilo/e2e-recovery:free');
    await expect(attemptChain).toContainText('成功');
    await expect(attemptChain).toContainText('2 步');
    if (page.viewportSize().width <= 768) {
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= window.innerWidth
        )
      ).toBe(true);
    }
  });
});
