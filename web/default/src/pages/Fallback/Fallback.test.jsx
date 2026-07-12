import { vi } from 'vitest';
import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import { MemoryRouter } from 'react-router-dom';
import Fallback from './index';
import { useFallbackPage } from './hooks/useFallbackPage';

/* global globalThis */

vi.mock('./hooks/useFallbackPage', () => ({
  useFallbackPage: vi.fn(),
}));

vi.mock('../../components/FallbackConfigPanel', () => ({ default: () => (
  <div>Gateway panel</div>
) }));

vi.mock('../../components/fallback-gateway/FreeModelPool', () => ({ default: () => (
  <div>Free pool panel</div>
) }));

vi.mock('./panels/SummaryBar', () => ({ default: () => <div>Summary bar</div> }));
vi.mock('./panels/StatusPanel', () => ({ default: () => <div>Status panel</div> }));
vi.mock('./panels/MetricsPanel', () => ({ default: () => <div>Metrics panel</div> }));
vi.mock('./panels/ScoresPanel', () => ({ default: () => <div>Scores panel</div> }));
vi.mock('./panels/AlertsPanel', () => ({ default: () => <div>Alerts panel</div> }));
vi.mock('./panels/LogsPanel', () => ({ default: () => <div>Logs panel</div> }));
vi.mock('./panels/KpiCards', () => ({ default: () => <div>Kpi cards</div> }));

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

const buildPageState = (overrides = {}) => ({
  activePanel: 'status',
  loading: false,
  lastUpdated: '2026-07-06T08:00:00Z',
  alertEvents: [],
  metricsText: '',
  switchEvents: [],
  statusSort: 'config',
  actingDeployment: '',
  guideOpen: true,
  summary: {},
  metricSamples: [],
  metricRows: [],
  configMeta: {},
  setStatusSort: vi.fn(),
  setGuideOpen: vi.fn(),
  statusDisplayRows: [],
  runtimeMetrics: {},
  runtimeHealth: {},
  metricTrendData: [],
  scoreTrend: [],
  scoreTrendGroups: [],
  loadPanel: vi.fn(),
  markAllAlertsRead: vi.fn(),
  runDeploymentAction: vi.fn(),
  exportMetricsCSV: vi.fn(),
  admin: true,
  refreshInterval: 15000,
  ...overrides,
});

const routerFuture = {
  v7_relativeSplatPath: true,
  v7_startTransition: true,
};

describe('Fallback shell', () => {
  let container;
  let root;

  beforeEach(() => {
    vi.clearAllMocks();
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => {
      root.unmount();
    });
    container.remove();
  });

  const renderFallback = async (state = {}) => {
    useFallbackPage.mockReturnValue(buildPageState(state));

    await act(async () => {
      root.render(
        <MemoryRouter
          initialEntries={[`/fallback/${state.activePanel || 'status'}`]}
          future={routerFuture}
        >
          <Fallback />
        </MemoryRouter>
      );
    });
  };

  test('renders the modern fallback shell and refresh action', async () => {
    const loadPanel = vi.fn();
    await renderFallback({ loadPanel });

    expect(container.textContent).toContain('Fallback 面板');
    expect(container.textContent).toContain('模型状态');
    expect(container.querySelectorAll('.fallback-nav-card')).toHaveLength(7);

    const refreshButton = container.querySelector(
      'button[aria-label="刷新当前面板"]'
    );
    expect(refreshButton).not.toBeNull();

    await act(async () => {
      refreshButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(loadPanel).toHaveBeenCalledWith(true);
  });

  test('shows the admin warning without rendering navigation', async () => {
    await renderFallback({ admin: false });

    expect(container.textContent).toContain(
      '需要管理员权限才能查看 fallback 面板。'
    );
    expect(container.querySelector('.fallback-panel-grid')).toBeNull();
  });

  test('shows a loading state for runtime panels only', async () => {
    await renderFallback({ activePanel: 'metrics', loading: true });

    expect(container.querySelector('.fallback-loading .ant-spin')).not.toBeNull();
    expect(container.textContent).not.toContain('Metrics panel');
  });

  test('keeps gateway and free-pool panels mounted through the shell', async () => {
    await renderFallback({ activePanel: 'free-pool', loading: true });
    expect(container.textContent).toContain('Free pool panel');

    await renderFallback({ activePanel: 'gateway', loading: true });
    expect(container.textContent).toContain('Gateway panel');
  });
});
