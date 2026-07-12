import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import { MemoryRouter } from 'react-router-dom';
import Fallback from './index';
import { useFallbackPage } from './hooks/useFallbackPage';

/* global globalThis */
/* eslint-disable testing-library/no-unnecessary-act */

jest.mock('./hooks/useFallbackPage', () => ({
  useFallbackPage: jest.fn(),
}));

jest.mock('../../components/FallbackConfigPanel', () => () => (
  <div>Gateway panel</div>
));

jest.mock('../../components/fallback-gateway/FreeModelPool', () => () => (
  <div>Free pool panel</div>
));

jest.mock('./panels/SummaryBar', () => () => <div>Summary bar</div>);
jest.mock('./panels/StatusPanel', () => () => <div>Status panel</div>);
jest.mock('./panels/MetricsPanel', () => () => <div>Metrics panel</div>);
jest.mock('./panels/ScoresPanel', () => () => <div>Scores panel</div>);
jest.mock('./panels/AlertsPanel', () => () => <div>Alerts panel</div>);
jest.mock('./panels/LogsPanel', () => () => <div>Logs panel</div>);
jest.mock('./panels/KpiCards', () => () => <div>Kpi cards</div>);

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
  setStatusSort: jest.fn(),
  setGuideOpen: jest.fn(),
  statusDisplayRows: [],
  runtimeMetrics: {},
  runtimeHealth: {},
  metricTrendData: [],
  scoreTrend: [],
  scoreTrendGroups: [],
  loadPanel: jest.fn(),
  markAllAlertsRead: jest.fn(),
  runDeploymentAction: jest.fn(),
  exportMetricsCSV: jest.fn(),
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
    jest.clearAllMocks();
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
    const loadPanel = jest.fn();
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
