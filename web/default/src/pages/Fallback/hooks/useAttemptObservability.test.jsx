import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import { vi } from 'vitest';
import { API } from '../../../helpers';
import { useAttemptObservability } from './useAttemptObservability';

vi.mock('../../../helpers', () => ({
  API: {
    get: vi.fn(),
  },
}));

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

const successfulPayload = {
  failure_event_count: 2,
  top_deployments: [{ deployment_id: 'free:kilo' }],
};

const Harness = ({ onChange }) => {
  onChange(useAttemptObservability());
  return null;
};

describe('useAttemptObservability', () => {
  let container;
  let root;
  let state;

  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    state = undefined;
  });

  afterEach(() => {
    act(() => {
      root.unmount();
    });
    container.remove();
    vi.useRealTimers();
  });

  test('loads immediately, normalizes absent arrays, and refreshes on demand', async () => {
    API.get.mockResolvedValueOnce({
      data: { success: true, data: successfulPayload },
    });

    await act(async () => {
      root.render(<Harness onChange={(nextState) => { state = nextState; }} />);
    });

    expect(API.get).toHaveBeenCalledTimes(1);
    expect(API.get).toHaveBeenCalledWith('/api/fallback/attempt-observability');
    expect(state).toMatchObject({
      data: {
        ...successfulPayload,
        top_providers: [],
        top_models: [],
        error_categories: [],
        outcomes: [],
        recent_chains: [],
      },
      error: '',
      loading: false,
    });

    API.get.mockResolvedValueOnce({
      data: { success: true, data: { failure_event_count: 3 } },
    });
    await act(async () => {
      await state.refresh();
    });

    expect(API.get).toHaveBeenCalledTimes(2);
    expect(state.data).toMatchObject({
      failure_event_count: 3,
      top_deployments: [],
      top_providers: [],
      top_models: [],
      error_categories: [],
      outcomes: [],
      recent_chains: [],
    });
  });

  test('retains the last successful data and shows a safe Chinese error after a polling failure', async () => {
    API.get
      .mockResolvedValueOnce({ data: { success: true, data: successfulPayload } })
      .mockRejectedValueOnce(new Error('upstream token should not be exposed'));

    await act(async () => {
      root.render(<Harness onChange={(nextState) => { state = nextState; }} />);
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30000);
    });

    expect(API.get).toHaveBeenCalledTimes(2);
    expect(state).toMatchObject({
      data: expect.objectContaining(successfulPayload),
      error: '精准尝试数据暂时不可用',
      loading: false,
    });
  });

  test('clears the polling timer when unmounted', async () => {
    API.get.mockResolvedValue({ data: { success: true, data: successfulPayload } });

    await act(async () => {
      root.render(<Harness onChange={(nextState) => { state = nextState; }} />);
    });
    expect(vi.getTimerCount()).toBe(1);

    act(() => {
      root.unmount();
    });

    expect(vi.getTimerCount()).toBe(0);
  });

  test('does not update state when a deferred request resolves after unmount', async () => {
    let resolveRequest;
    const request = new Promise((resolve) => {
      resolveRequest = resolve;
    });
    const onChange = vi.fn();
    API.get.mockReturnValueOnce(request);

    await act(async () => {
      root.render(<Harness onChange={onChange} />);
    });
    const renderCountAtUnmount = onChange.mock.calls.length;

    act(() => {
      root.unmount();
    });
    await act(async () => {
      resolveRequest({ data: { success: true, data: successfulPayload } });
      await request;
    });

    expect(onChange).toHaveBeenCalledTimes(renderCountAtUnmount);
  });
});
