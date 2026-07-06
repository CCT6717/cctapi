import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import FreeModelPool from './FreeModelPool';
import {
  getFreePoolUsage,
  getGatewayConfig,
  getRuntimeStatus,
} from './gatewayConfigApi';

jest.mock('../../helpers', () => ({
  showError: jest.fn(),
  showSuccess: jest.fn(),
}));

jest.mock('./gatewayConfigApi', () => ({
  cleanupDryRun: jest.fn(),
  getFreePoolUsage: jest.fn(),
  getGatewayConfig: jest.fn(),
  getRuntimeStatus: jest.fn(),
  reloadConfig: jest.fn(),
  saveGatewayConfig: jest.fn(),
  syncFreePool: jest.fn(),
}));

jest.mock('semantic-ui-react', () => {
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
    Table,
  };
});

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

describe('FreeModelPool', () => {
  let container;
  let root;

  beforeEach(() => {
    jest.clearAllMocks();
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

    const panels = container.querySelectorAll('section.fallback-virtual-panel');
    expect(panels.length).toBe(4);
    expect(panels[2].querySelector('tbody tr div')).not.toBeNull();
    expect(panels[2].querySelectorAll('tbody tr').length).toBe(1);
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

    expect(container.querySelector('.free-pool-workflow-dashboard')).not.toBeNull();
    expect(container.querySelector('.free-pool-readiness-meter strong')?.textContent).toMatch(/^0\/4/);
    expect(container.querySelectorAll('.free-pool-workflow-step.blocked').length).toBeGreaterThan(0);
    expect(container.querySelectorAll('.free-pool-workflow-actions-panel li').length).toBeGreaterThan(0);
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
    const searchInput = container.querySelector('.free-provider-filter-row input');
    expect(ops).not.toBeNull();
    expect(searchInput).not.toBeNull();
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

    expect(container.textContent).toContain('Groq');
    expect(container.querySelector('.free-provider-count strong')?.textContent).toBe('1');
    expect(container.querySelectorAll('tr.free-provider-row').length).toBe(1);
    expect(container.querySelector('tr.free-provider-row.is-enabled.needs-key')).not.toBeNull();

    const bulkButtons = Array.from(
      container.querySelectorAll('.free-provider-bulk-row button'),
    );

    await act(async () => {
      const selectVisibleButton = bulkButtons[0];
      expect(selectVisibleButton).toBeDefined();
      selectVisibleButton.click();
    });

    await act(async () => {
      const disableSelectedButton = bulkButtons[3];
      expect(disableSelectedButton).toBeDefined();
      disableSelectedButton.click();
    });

    expect(container.querySelector('.free-provider-selected-count')?.textContent).toContain('1');
    expect(container.querySelector('tr.free-provider-row.is-disabled.needs-key')).not.toBeNull();
  });
});
