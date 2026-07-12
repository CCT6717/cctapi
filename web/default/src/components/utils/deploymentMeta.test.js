import { vi } from 'vitest';
import {
  computeInitialMode,
  getVirtualModelDeploymentIds,
  applyVmModeSelection,
} from './deploymentMeta';

describe('computeInitialMode', () => {
  test('treats preferred deployment in fallback mode as fixed only for the current vm', () => {
    const data = {
      virtual_models: {
        'cct/high': {
          routing_mode: 'fallback',
          preferred_deployment: 'dep-b',
          fallback_order: ['dep-a', 'dep-b'],
        },
        'cct/other': {
          routing_mode: 'fallback',
          preferred_deployment: '',
          fallback_order: ['dep-b'],
        },
      },
      deployments: {
        'dep-a': { pool: '_fixed_legacy-a', daily_limit_tokens: 100000 },
        'dep-b': { pool: '_fixed_legacy-b', daily_limit_tokens: 0 },
      },
    };

    expect(computeInitialMode(data, 'dep-a', 'cct/high')).toBe('quota');
    expect(computeInitialMode(data, 'dep-b', 'cct/high')).toBe('fixed');
    expect(computeInitialMode(data, 'dep-b', 'cct/other')).toBe('error');
  });
});

describe('getVirtualModelDeploymentIds', () => {
  test('prefers fallback_order over pools when determining vm ownership', () => {
    const deployments = {
      'dep-a': { pool: '_fixed_pool_a' },
      'dep-b': { pool: '_fixed_pool_b' },
      'dep-c': { pool: '_fixed_pool_b' },
    };

    const vm = {
      pools: ['_fixed_pool_b'],
      fallback_order: ['dep-a', 'dep-c'],
    };

    expect(getVirtualModelDeploymentIds(vm, deployments)).toEqual(['dep-a', 'dep-c']);
  });
});

describe('applyVmModeSelection', () => {
  test('clears config-derived fixed mode in the same vm when selecting a new fixed deployment', () => {
    const data = {
      virtual_models: {
        'cct/high': {
          routing_mode: 'fallback',
          preferred_deployment: 'dep-c',
          fallback_order: ['dep-a', 'dep-b', 'dep-c'],
          pools: ['pool-x'],
        },
      },
      deployments: {
        'dep-a': { pool: 'pool-x', daily_limit_tokens: 0 },
        'dep-b': { pool: 'pool-x', daily_limit_tokens: 0 },
        'dep-c': { pool: 'pool-x', daily_limit_tokens: 0 },
      },
    };

    expect(applyVmModeSelection({
      previousModes: {},
      depId: 'dep-b',
      mode: 'fixed',
      vmKey: 'cct/high',
      data,
    })).toEqual({
      'dep-b': 'fixed',
      'dep-c': 'error',
    });
  });
});
