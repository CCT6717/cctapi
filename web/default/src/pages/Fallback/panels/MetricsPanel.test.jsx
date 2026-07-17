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

const emptyAttemptData = {
  failure_event_count: 0,
  skip_event_count: 0,
  top_deployments: [],
  top_providers: [],
  top_models: [],
  error_categories: [],
  outcomes: [],
  recent_chains: [],
  recent_chain_scope: 'process',
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
    vi.restoreAllMocks();
  });

  const renderPanel = () => {
    act(() => {
      root.render(<MetricsPanel {...props} />);
    });
  };

  test('shows precise failures, separate skips, safe route chains, and existing switch counts', () => {
    renderPanel();

    const text = container.textContent;
    expect(text).toContain('精准失败诊断');
    expect(text).toContain('最近请求链路');
    expect(text).toContain('真实上游失败');
    expect(text).toContain('本地跳过');
    expect(text).toContain('kilo/model-a:free');
    expect(text).toContain('kilo/model-b:free');
    expect(text).toContain('成功');
    expect(text).toContain('HTTP 429');
    expect(text).toContain('模型限速');
    expect(text).toContain('配额跳过');
    expect(text).toContain('错误类别限速1 次');
    expect(
      container.querySelector('.fallback-attempt-stat.skipped').textContent
    ).toContain('本地跳过配额跳过 1 次1');
    expect(text).toContain('切换次数7近 1 小时 4 次');
    expect(text).not.toContain('Top 失败模型/渠道');
    expect(text).not.toContain(unsafeError);
    expect(text).not.toContain(unsafeToken);
  });

  test('does not render the legacy approximate deployment ranking', () => {
    const legacySentinel = 'LEGACY_APPROXIMATE_RANKING_SENTINEL';
    act(() => {
      root.render(
        <MetricsPanel
          {...props}
          runtimeHealth={{
            ...props.runtimeHealth,
            topDeploymentFailures: [
              {
                key: 'legacy-ranking',
                deployment: legacySentinel,
                model: 'legacy-model',
                count: 99,
              },
            ],
          }}
        />
      );
    });

    expect(container.textContent).not.toContain('Top 3 失败模型');
    expect(container.textContent).not.toContain(legacySentinel);
  });

  test('renders every supported local skip outcome label', () => {
    useAttemptObservability.mockReturnValue({
      data: {
        ...emptyAttemptData,
        skip_event_count: 3,
        outcomes: [
          { outcome: 'skipped_concurrency', count: 1 },
          { outcome: 'skipped_channel', count: 1 },
          { outcome: 'skipped_model_state', count: 1 },
        ],
      },
      error: '',
      loading: false,
      refresh: vi.fn(),
    });

    renderPanel();

    expect(container.textContent).toContain('并发跳过 1 次');
    expect(container.textContent).toContain('渠道跳过 1 次');
    expect(container.textContent).toContain('模型状态跳过 1 次');
  });

  test('shows one safe hook error when no precise snapshot is available', () => {
    useAttemptObservability.mockReturnValue({
      data: null,
      error: '精准尝试数据暂时不可用',
      loading: false,
      refresh: vi.fn(),
    });

    renderPanel();

    expect(container.textContent.match(/精准尝试数据暂时不可用/g)).toHaveLength(
      1
    );
    expect(container.textContent).toContain('精准失败诊断');
  });

  test('renders independent empty states for all aggregates and recent chains', () => {
    useAttemptObservability.mockReturnValue({
      data: emptyAttemptData,
      error: '',
      loading: false,
      refresh: vi.fn(),
    });

    renderPanel();

    const text = container.textContent;
    expect(text).toContain('真实上游失败已实际发往上游的失败尝试0');
    expect(text).toContain('本地跳过未调用上游的本地路由判断0');
    expect(text).toContain('暂无真实部署失败');
    expect(text).toContain('暂无提供商失败');
    expect(text).toContain('暂无真实模型失败');
    expect(text).toContain('暂无错误类别');
    expect(text).toContain('暂无最近请求链路');
  });

  test('uses collision-free chain keys and accessible ordered step lists', () => {
    const duplicateChain = {
      request_id: 'request-duplicate',
      virtual_model: 'openrouter/auto',
      started_at: '2026-07-17T03:00:00Z',
      finished_at: '2026-07-17T03:00:01Z',
      steps: attemptData.recent_chains[0].steps,
    };
    const missingIDChain = {
      virtual_model: 'openrouter/auto',
      started_at: '2026-07-17T03:01:00Z',
      finished_at: '2026-07-17T03:01:01Z',
      steps: attemptData.recent_chains[0].steps,
    };
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
    useAttemptObservability.mockReturnValue({
      data: {
        ...emptyAttemptData,
        recent_chains: [
          duplicateChain,
          { ...duplicateChain },
          missingIDChain,
          { ...missingIDChain },
        ],
      },
      error: '',
      loading: false,
      refresh: vi.fn(),
    });

    renderPanel();

    const chainArticles = container.querySelectorAll(
      'article.fallback-attempt-chain-card[aria-label]'
    );
    expect(chainArticles).toHaveLength(4);
    chainArticles.forEach((article) => {
      expect(article.getAttribute('aria-label')).not.toBe('');
      expect(article.querySelector('ol.fallback-attempt-steps')).not.toBeNull();
      expect(article.querySelectorAll('ol.fallback-attempt-steps > li')).toHaveLength(
        2
      );
    });
    expect(
      consoleError.mock.calls.some((call) =>
        /same key|unique.*key/i.test(call.map(String).join(' '))
      )
    ).toBe(false);
  });

  test('keeps the chain article mounted as completion fields and steps update', () => {
    const initialChain = {
      ...attemptData.recent_chains[0],
      finished_at: '2026-07-17T03:00:00.500Z',
      steps: [attemptData.recent_chains[0].steps[0]],
    };
    useAttemptObservability.mockReturnValue({
      data: {
        ...emptyAttemptData,
        recent_chains: [initialChain],
      },
      error: '',
      loading: false,
      refresh: vi.fn(),
    });

    renderPanel();

    const initialArticle = container.querySelector(
      'article.fallback-attempt-chain-card'
    );
    expect(initialArticle.textContent).toContain('kilo/model-a:free');
    expect(initialArticle.textContent).not.toContain('kilo/model-b:free');

    useAttemptObservability.mockReturnValue({
      data: {
        ...emptyAttemptData,
        recent_chains: [
          {
            ...initialChain,
            finished_at: '2026-07-17T03:00:01Z',
            steps: attemptData.recent_chains[0].steps,
          },
        ],
      },
      error: '',
      loading: false,
      refresh: vi.fn(),
    });

    renderPanel();

    const updatedArticle = container.querySelector(
      'article.fallback-attempt-chain-card'
    );
    expect(updatedArticle).toBe(initialArticle);
    expect(updatedArticle.textContent).toContain('kilo/model-b:free');
    expect(updatedArticle.textContent).toContain('成功');
  });
});
