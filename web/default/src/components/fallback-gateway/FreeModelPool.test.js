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
  const passthrough = (tag) => ({ children }) => React.createElement(tag, null, children);
  const Button = ({ children, disabled, onClick }) => (
    <button type='button' disabled={disabled} onClick={onClick}>
      {children}
    </button>
  );
  const Header = passthrough('h2');
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
  Table.Row = passthrough('tr');
  Table.HeaderCell = passthrough('th');
  Table.Cell = ({ children, colSpan }) => <td colSpan={colSpan}>{children}</td>;

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
    Label: passthrough('span'),
    Loader: () => <div>Loading</div>,
    Message: ({ children, content, header }) => (
      <div>
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

    expect(container.textContent).toContain('Usage data unavailable');
    expect(container.textContent).toContain('Usage data is unavailable.');
    expect(container.textContent).not.toContain('No usage rows for the selected period');
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

    expect(container.textContent).toContain('Integration readiness');
    expect(container.textContent).toContain('0/4 complete');
    expect(container.textContent).toContain('Enable cct/free virtual model.');
    expect(container.textContent).toContain('Sync the free pool to generate deployments.');
  });

  test('filters provider rows and stages bulk disable for selected visible providers', async () => {
    getGatewayConfig.mockResolvedValue({
      data: {
        success: true,
        data: {
          free_providers: {
            groq: { enabled: true, key_count: 2 },
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

    await act(async () => {
      const searchInput = container.querySelector('input[aria-label="Search free providers"]');
      expect(searchInput).not.toBeNull();
      const setValue = Object.getOwnPropertyDescriptor(
        window.HTMLInputElement.prototype,
        'value',
      ).set;
      setValue.call(searchInput, 'groq');
      searchInput.dispatchEvent(new Event('input', { bubbles: true }));
    });

    expect(container.textContent).toContain('Groq');
    expect(container.textContent).not.toContain('openrouterconfigured');

    await act(async () => {
      const selectVisibleButton = Array.from(container.querySelectorAll('button'))
        .find((button) => button.textContent.includes('Select visible'))
      expect(selectVisibleButton).toBeDefined();
      selectVisibleButton.click();
    });

    await act(async () => {
      const disableSelectedButton = Array.from(container.querySelectorAll('button'))
        .find((button) => button.textContent.includes('Disable selected'))
      expect(disableSelectedButton).toBeDefined();
      disableSelectedButton.click();
    });

    expect(container.textContent).toContain('Staged 1 provider');
    expect(container.textContent).toContain('disabled');
    expect(container.textContent).not.toContain('openrouterconfigured');
  });
});
