import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import FallbackRuntimePanel from './FallbackRuntimePanel';
import { API } from '../helpers';

/* global globalThis */
/* eslint-disable testing-library/no-unnecessary-act */

jest.mock('../helpers', () => ({
  API: {
    get: jest.fn(),
    post: jest.fn(),
  },
  showError: jest.fn(),
  showSuccess: jest.fn(),
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
  Button.Group = passthrough('div');
  const Table = passthrough('table');
  Table.Header = passthrough('thead');
  Table.Body = passthrough('tbody');
  Table.Row = passthrough('tr');
  Table.HeaderCell = passthrough('th');
  Table.Cell = ({ children, colSpan, className, textAlign }) => (
    <td className={className} colSpan={colSpan} data-text-align={textAlign}>
      {children}
    </td>
  );

  return {
    Button,
    Icon: () => null,
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

describe('FallbackRuntimePanel', () => {
  let container;
  let root;

  beforeEach(() => {
    jest.clearAllMocks();
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    API.get.mockImplementation((url) => {
      if (url === '/api/fallback/virtual-models') {
        return Promise.resolve({ data: { success: true, data: [] } });
      }
      if (url === '/api/fallback/deployments/runtime-status') {
        return Promise.resolve({
          data: {
            success: true,
            data: [{
              deployment_id: 'free:groq-runtime-visible',
              pool: 'free',
              real_model: 'llama-free',
              health: 'rate_limited',
              minute_requests: 1,
              rpm_limit: 10,
              day_requests: 2,
              rpd_limit: 20,
              minute_tokens: 3,
              tpm_limit: 30,
              day_tokens: 4,
              tpd_limit: 40,
              cooldown_active: true,
              cooldown_reason: 'rate limited by provider',
              exhausted_active: true,
              state_last_error_message: 'daily quota exceeded',
              is_sticky: true,
              sticky_virtual_models: ['cct/free'],
            }],
          },
        });
      }
      return Promise.reject(new Error(`unexpected url ${url}`));
    });
  });

  afterEach(() => {
    act(() => {
      root.unmount();
    });
    container.remove();
  });

  test('renders runtime diagnostics for deployments', async () => {
    await act(async () => {
      root.render(<FallbackRuntimePanel />);
    });

    expect(container.textContent).toContain('Sticky');
    expect(container.textContent).toContain('cct/free');
    expect(container.textContent).toContain('Cooldown');
    expect(container.textContent).toContain('rate limited by provider');
    expect(container.textContent).toContain('Exhausted');
    expect(container.textContent).toContain('daily quota exceeded');
  });
});
