import { buildSavePayload } from './savePipeline';
describe('buildSavePayload', () => {
  test('fixed mode keeps fallback routing while pinning the preferred deployment', () => {
    const fresh = {
      virtual_models: {
        'cct/high': {
          routing_mode: 'fallback',
          preferred_deployment: '',
          pools: ['paid_high'],
          allow_degrade_to_low: true,
          allow_degrade_to_free: true,
        },
      },
      deployments: {
        'dep-a': { pool: 'paid_high', enabled: true },
        'dep-b': { pool: 'paid_high', enabled: true, daily_limit_tokens: 100000 },
      },
    };

    const { payload } = buildSavePayload(fresh, {
      draftDeployments: {},
      draftRoutingVm: {},
      deploymentMode: { 'dep-b': 'fixed' },
      deploymentOwnerVm: { 'dep-a': ['cct/high'], 'dep-b': ['cct/high'] },
    });

    expect(payload.virtual_models['cct/high'].routing_mode).toBe('fallback');
    expect(payload.virtual_models['cct/high'].preferred_deployment).toBe('dep-b');
    expect(payload.virtual_models['cct/high'].allow_degrade_to_low).toBe(false);
    expect(payload.virtual_models['cct/high'].allow_degrade_to_free).toBe(false);
    expect(payload.deployments['dep-b'].daily_limit_tokens).toBe(0);
  });
});
