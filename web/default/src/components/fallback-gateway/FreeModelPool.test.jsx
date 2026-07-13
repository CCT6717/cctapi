import { vi } from 'vitest';
import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import FreeModelPool from './FreeModelPool';
import { showSuccess, showWarning } from '../../helpers';
import {
  getFreePoolUsage,
  getGatewayConfig,
  getRuntimeStatus,
  syncFreePool,
} from './gatewayConfigApi';

/* global globalThis */

vi.mock('../../helpers', () => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
  showWarning: vi.fn(),
}));

vi.mock('./gatewayConfigApi', () => ({
  cleanupDryRun: vi.fn(),
  getFreePoolUsage: vi.fn(),
  getGatewayConfig: vi.fn(),
  getRuntimeStatus: vi.fn(),
  reloadConfig: vi.fn(),
  saveGatewayConfig: vi.fn(),
  syncFreePool: vi.fn(),
}));

vi.mock('semantic-ui-react', () => {
  const React = require('react');
  const passthrough = (tag) => ({ children, className }) =>
    React.createElement(tag, className ? { className } : null, children);
  const Button = ({ children, disabled, onClick }) => (
    <button type='button' disabled={disabled} onClick={onClick}>
      {children}
    </button>
  );
  const Header = ({ children, className }) => <h2 className={className}>{children}</h2>;
  Header.Subheader = passthrough('div');
  const Form = passthrough('form');
  Form.Group = passthrough('div');
  Form.Field = passthrough('div');
  Form.TextArea = ({ value = '', onChange }) => (
    <textarea value={value} onChange={(event) => onChange?.(event, { value: event.target.value })} />
  );
  const Table = passthrough('table');
  Table.Header = passthrough('thead');
  Table.Body = passthrough('tbody');
  Table.Row = ({ children, className }) => <tr className={className}>{children}</tr>;
  Table.HeaderCell = passthrough('th');
  Table.Cell = ({ children, colSpan, textAlign }) => <td colSpan={colSpan} data-text-align={textAlign}>{children}</td>;

  return {
    Button,
    Checkbox: ({ checked, onChange }) => (
      <input
        type='checkbox'
        checked={checked}
        onChange={(event) => onChange?.(event, { checked: event.target.checked })}
      />
    ),
    Form,
    Header,
    Icon: () => null,
    Input: ({ 'aria-label': ariaLabel, placeholder, value = '', onChange }) => (
      <input
        aria-label={ariaLabel}
        placeholder={placeholder}
        value={value}
        onChange={(event) => onChange?.(event, { value: event.target.value })}
      />
    ),
    Label: ({ children, className }) => <span className={className}>{children}</span>,
    Loader: () => <div>Loading</div>,
    Message: ({ children, content, header, className }) => (
      <div className={className}>
        {header}
        {content}
        {children}
      </div>
    ),
    Popup: ({ content, trigger }) => <span data-tooltip={content}>{trigger}</span>,
    Table,
  };
});

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

const findButtonByText = (scope, label) => Array.from(scope.querySelectorAll('button'))
  .find((button) => button.textContent.trim() === label);

describe('FreeModelPool', () => {
  let container;
  let root;

  beforeEach(() => {
    vi.clearAllMocks();
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    getGatewayConfig.mockResolvedValue({
      data: {
        success: true,
        data: {
          free_providers: {},
          free_provider_catalog: [],
          virtual_models: {
            'cct/free': {
              enabled: true,
              pools: ['free'],
              strategy: 'free_first',
            },
          },
          deployments: {},
        },
      },
    });
    getRuntimeStatus.mockResolvedValue({
      data: { success: true, data: [] },
    });
  });

  afterEach(() => {
    act(() => {
      root.unmount();
    });
    container.remove();
  });

  test('renders usage unavailable state instead of the no-usage row', async () => {
    getFreePoolUsage.mockResolvedValue({
      data: { success: false, message: 'sanitized usage lookup failed' },
    });

    await act(async () => {
      root.render(<FreeModelPool />);
    });

    expect(container.textContent).toContain('用量数据不可用');
    expect(container.textContent).toContain('用量数据不可用。');
    expect(container.textContent).not.toContain('当前周期暂无用量记录');
  });

  test('renders workflow readiness and recommended next actions', async () => {
    getGatewayConfig.mockResolvedValue({
      data: {
        success: true,
        data: {
          free_providers: {
            groq: { enabled: true, key_count: 0 },
          },
          free_provider_catalog: [{ name: 'groq', requires_key: true }],
          virtual_models: {
            'cct/free': {
              enabled: false,
              pools: ['free'],
              strategy: 'free_first',
            },
          },
          deployments: {},
        },
      },
    });
    getFreePoolUsage.mockResolvedValue({
      data: { success: false, message: 'usage unavailable' },
    });

    await act(async () => {
      root.render(<FreeModelPool />);
    });

    const workflowText = container.textContent;
    const actionItems = Array.from(
      container.querySelectorAll('.free-pool-workflow-actions-panel li'),
    ).map((item) => item.textContent?.trim());

    expect(container.querySelector('.free-pool-workflow-dashboard')).not.toBeNull();
    expect(workflowText).toContain('接入就绪度');
    expect(workflowText).toContain('建议下一步');
    expect(container.querySelector('.free-pool-readiness-meter strong')?.textContent).toMatch(/^0\/4/);
    expect(container.querySelector('.free-pool-status-strip.warning strong')?.textContent).toContain('需要处理');
    expect(container.querySelector('.free-pool-status-strip.warning span')?.textContent)
      .toContain('cct/free');
    expect(container.querySelectorAll('.free-pool-workflow-step.blocked').length).toBeGreaterThan(0);
    expect(actionItems.length).toBeGreaterThan(0);
    expect(actionItems).toEqual(expect.arrayContaining([
      '启用 cct/free 虚拟模型。',
      '同步免费池以生成部署。',
    ]));
  });

  test('renders catalog details in the existing provider status cell', async () => {
    getGatewayConfig.mockResolvedValue({
      data: {
        success: true,
        data: {
          free_providers: {
            ovh: { enabled: true, keyless: true },
          },
          free_provider_catalog: [{
            name: 'ovh',
            keyless: true,
            catalog_status: {
              refreshable: true,
              source: 'openai_models',
              model_count: 12,
              last_attempt_at: '2026-07-13T13:00:00Z',
              last_success_at: '2026-07-13T12:00:00Z',
              stale: true,
              last_error: 'catalog endpoint unavailable',
            },
          }],
          virtual_models: {
            'cct/free': {
              enabled: true,
              pools: ['free'],
              strategy: 'free_first',
            },
          },
          deployments: {},
        },
      },
    });
    getFreePoolUsage.mockResolvedValue({
      data: { success: true, data: [] },
    });

    await act(async () => {
      root.render(<FreeModelPool />);
    });

    const providerRow = container.querySelector('tr.free-provider-row');
    expect(providerRow).not.toBeNull();
    expect(providerRow.textContent).toContain('刷新失败');
    expect(providerRow.textContent).toContain('12 个模型');
    expect(providerRow.textContent).toContain('最后成功');
    expect(providerRow.textContent).toContain('2026');
    expect(providerRow.querySelector('[data-tooltip="catalog endpoint unavailable"]')).not.toBeNull();
  });

  test('warns instead of reporting success when catalog sync is partially failed', async () => {
    getFreePoolUsage.mockResolvedValue({
      data: { success: true, data: [] },
    });
    syncFreePool.mockResolvedValue({
      data: {
        success: false,
        data: {
          catalog_sync: {
            attempted: 3,
            succeeded: 2,
            failed: 1,
            results: [{ provider: 'ovh', success: false, error: 'upstream unavailable' }],
          },
        },
      },
    });

    await act(async () => {
      root.render(<FreeModelPool />);
    });

    const syncButton = findButtonByText(container, '同步并刷新目录');
    expect(syncButton).toBeDefined();

    await act(async () => {
      syncButton.click();
      await Promise.resolve();
      await Promise.resolve();
    });

    await vi.waitFor(() => {
      expect(showWarning).toHaveBeenCalledWith(expect.stringContaining('1 个目录刷新失败'));
    });
    expect(showSuccess).not.toHaveBeenCalled();
  });

  test('filters provider rows and stages bulk disable for selected visible providers', async () => {
    getGatewayConfig.mockResolvedValue({
      data: {
        success: true,
        data: {
          free_providers: {
            groq: { enabled: true, key_count: 0 },
            openrouter: { enabled: true, key_count: 1 },
          },
          free_provider_catalog: [
            { name: 'groq', requires_key: true, supports_tools: true },
            { name: 'openrouter', requires_key: true, supports_json: true },
          ],
          virtual_models: {
            'cct/free': {
              enabled: true,
              pools: ['free'],
              strategy: 'free_first',
            },
          },
          deployments: {},
        },
      },
    });
    getFreePoolUsage.mockResolvedValue({
      data: { success: true, data: [] },
    });

    await act(async () => {
      root.render(<FreeModelPool />);
    });

    const ops = container.querySelector('.free-provider-ops');
    expect(ops).not.toBeNull();
    const searchInput = ops.querySelector('input[aria-label="搜索免费供应商"]');
    expect(searchInput).not.toBeNull();
    expect(searchInput.getAttribute('placeholder')).toContain('搜索供应商');

    const selectVisibleButton = findButtonByText(ops, '选择可见项');
    const enableSelectedButton = findButtonByText(ops, '批量启用');
    const disableSelectedButton = findButtonByText(ops, '批量停用');
    expect(selectVisibleButton).toBeDefined();
    expect(enableSelectedButton).toBeDefined();
    expect(disableSelectedButton).toBeDefined();
    expect(container.textContent).toContain('Groq');
    expect(container.textContent).toContain('OpenRouter');

    await act(async () => {
      const setValue = Object.getOwnPropertyDescriptor(
        window.HTMLInputElement.prototype,
        'value',
      ).set;
      setValue.call(searchInput, 'groq');
      searchInput.dispatchEvent(new Event('input', { bubbles: true }));
    });

    const providerRows = Array.from(container.querySelectorAll('tr.free-provider-row'));
    expect(container.textContent).toContain('Groq');
    expect(providerRows.map((row) => row.textContent).join(' ')).not.toContain('OpenRouter');
    expect(container.querySelector('.free-provider-count strong')?.textContent).toBe('1');
    expect(providerRows.length).toBe(1);
    expect(container.querySelector('tr.free-provider-row.is-enabled.needs-key')).not.toBeNull();

    await act(async () => {
      selectVisibleButton.click();
    });

    await act(async () => {
      disableSelectedButton.click();
    });

    expect(container.querySelector('.free-provider-selected-count')?.textContent).toContain('已选择 1 项');
    expect(container.textContent).toContain('已停用');
    expect(container.querySelector('tr.free-provider-row.is-disabled.needs-key')).not.toBeNull();
  });

  test('renders runtime diagnostics for free deployments', async () => {
    getGatewayConfig.mockResolvedValue({
      data: {
        success: true,
        data: {
          free_providers: {},
          free_provider_catalog: [],
          virtual_models: {
            'cct/free': {
              enabled: true,
              pools: ['free'],
              strategy: 'free_first',
            },
          },
          deployments: {
            'free:groq-runtime-visible': {
              enabled: true,
              pool: 'free',
              real_model: 'llama-free',
              quota_mode: 'free',
            },
          },
        },
      },
    });
    getRuntimeStatus.mockResolvedValue({
      data: {
        success: true,
        data: [{
          deployment_id: 'free:groq-runtime-visible',
          health: 'rate_limited',
          cooldown_active: true,
          cooldown_reason: 'rate limited by provider',
          exhausted_active: true,
          state_last_error_message: 'daily quota exceeded',
          is_sticky: true,
          sticky_virtual_models: ['cct/free'],
        }],
      },
    });
    getFreePoolUsage.mockResolvedValue({
      data: { success: true, data: [] },
    });

    await act(async () => {
      root.render(<FreeModelPool />);
    });

    expect(container.textContent).toContain('Sticky');
    expect(container.textContent).toContain('cct/free');
    expect(container.textContent).toContain('Cooldown');
    expect(container.textContent).toContain('rate limited by provider');
    expect(container.textContent).toContain('Exhausted');
    expect(container.textContent).toContain('daily quota exceeded');
  });
});
