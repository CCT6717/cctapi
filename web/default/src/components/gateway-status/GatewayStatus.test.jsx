import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import { vi } from 'vitest';
import GatewayStatus from './GatewayStatus';
import { API } from '../../helpers';

vi.mock('../../helpers', () => ({
  API: {
    get: vi.fn(),
    post: vi.fn(),
  },
  showError: vi.fn(),
  showSuccess: vi.fn(),
}));

vi.mock('semantic-ui-react', () => ({
  Loader: () => <div>加载中</div>,
}));

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

describe('GatewayStatus', () => {
  let container;
  let root;

  beforeEach(() => {
    vi.clearAllMocks();
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    API.get.mockImplementation((url) => {
      if (url === '/api/fallback/virtual-models') {
        return Promise.resolve({
          data: {
            success: true,
            data: [{
              name: 'free/auto',
              strategy: 'free_first',
              enabled: true,
              pools: ['free'],
            }],
          },
        });
      }
      if (url === '/api/fallback/deployments/runtime-status') {
        return Promise.resolve({
          data: {
            success: true,
            data: [
              {
                deployment_id: 'free:active',
                pool: 'free',
                health: 'rate_limited',
                provider_rate_limit_degradation: {
                  active: true,
                  level: 2,
                  episode_count: 3,
                  next_recovery_at: '2026-07-16T08:30:00Z',
                },
              },
              {
                deployment_id: 'free:inactive',
                pool: 'free',
                health: 'healthy',
                provider_rate_limit_degradation: {
                  active: false,
                  level: 1,
                  episode_count: 1,
                },
              },
            ],
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

  test('renders active provider rate-limit degradation without inactive diagnostics', async () => {
    await act(async () => {
      root.render(<GatewayStatus />);
    });

    await act(async () => {
      container.querySelector('.gw-row-top').click();
    });

    expect(container.textContent).toContain('持续限流降权 L2');
    expect(container.textContent).toContain('跨冷却限流 3 次');
    expect(container.textContent).toContain('预计恢复');

    const inactiveCard = [...container.querySelectorAll('.gw-dep-card')].find((card) =>
      card.textContent.includes('free:inactive'),
    );
    expect(inactiveCard).not.toBeNull();
    expect(inactiveCard?.querySelector('.gw-degradation')).toBeNull();
  });
});
