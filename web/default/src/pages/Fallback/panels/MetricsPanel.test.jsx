import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import { vi } from 'vitest';
import { useAttemptObservability } from '../hooks/useAttemptObservability';
import MetricsPanel from './MetricsPanel';

vi.mock('../hooks/useAttemptObservability', () => ({
  useAttemptObservability: vi.fn(),
}));

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

const unsafeError = 'INJECTED_RAW_ERROR_SENTINEL';
const unsafeToken = 'sk-injected-token-sentinel';

const attemptData = {
  failure_event_count: 1,
  skip_event_count: 1,
  top_deployments: [
    {
      key: 'free:kilo',
      deployment_id: 'free:kilo',
      count: 1,
      raw_error: unsafeError,
    },
  ],
  top_providers: [{ key: 'kilo', provider: 'kilo', count: 1 }],
  top_models: [
    { key: 'kilo/model-a:free', real_model: 'kilo/model-a:free', count: 1 },
  ],
  error_categories: [
    { key: 'rate_limit', category: 'rate_limit', count: 1 },
  ],
  outcomes: [
    { key: 'skipped_quota', outcome: 'skipped_quota', count: 1 },
  ],
  recent_chains: [
    {
      request_id: 'request-safe-1',
      virtual_model: 'openrouter/auto',
      started_at: '2026-07-17T03:00:00Z',
      finished_at: '2026-07-17T03:00:01Z',
      steps: [
        {
          provider: 'kilo',
          deployment_id: 'free:kilo',
          real_model: 'kilo/model-a:free',
          outcome: 'model_rate_limited',
          status_code: 429,
          error_category: 'rate_limit',
          duration_ms: 320,
          plan_index: 0,
          upstream_attempt_index: 1,
          raw_error: unsafeError,
          token: unsafeToken,
        },
        {
          provider: 'kilo',
          deployment_id: 'free:kilo',
          real_model: 'kilo/model-b:free',
          outcome: 'success',
          status_code: 200,
          error_category: 'none',
          duration_ms: 410,
          plan_index: 0,
          upstream_attempt_index: 2,
        },
      ],
    },
  ],
  recent_chain_scope: 'process',
  token: unsafeToken,
};

const props = {
  runtimeMetrics: {
    requests: 12,
    switches: 7,
    success: 11,
    failed: 1,
    successRate: 91.7,
    totalTokens: 0,
    tokenRows: [],
    maxDeploymentTokens: 0,
  },
  metricTrendData: [],
  runtimeHealth: {
    level: 'normal',
    title: '运行正常',
    message: '当前没有明显异常。',
    recentSwitchCount: 4,
    fiveMinuteRate: { successRate: 100, total: 2 },
    oneHourRate: { failureRate: 8.3, total: 12 },
    coolingRows: [],
    quotaExhaustedRows: [],
    topDeploymentFailures: [],
    topModelFailures: [],
    topChannelFailures: [],
  },
  metricSamples: [],
  metricsText: '',
  metricRows: [],
  summary: { switch_count: 4 },
  exportMetricsCSV: vi.fn(),
};

describe('MetricsPanel precise attempt diagnostics', () => {
  let container;
  let root;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    useAttemptObservability.mockReturnValue({
      data: attemptData,
      error: '',
      loading: false,
      refresh: vi.fn(),
    });
  });

  afterEach(() => {
    act(() => {
      root.unmount();
    });
    container.remove();
    vi.clearAllMocks();
  });

  test('shows precise failures, separate skips, safe route chains, and existing switch counts', () => {
    act(() => {
      root.render(<MetricsPanel {...props} />);
    });

    const text = container.textContent;
    expect(text).toContain('精准失败诊断');
    expect(text).toContain('最近请求链路');
    expect(text).toContain('真实上游失败');
    expect(text).toContain('本地跳过');
    expect(text).toContain('kilo/model-a:free');
    expect(text).toContain('kilo/model-b:free');
    expect(text).toContain('成功');
    expect(text).toContain('切换次数7近 1 小时 4 次');
    expect(text).not.toContain('Top 失败模型/渠道');
    expect(text).not.toContain(unsafeError);
    expect(text).not.toContain(unsafeToken);
  });
});
