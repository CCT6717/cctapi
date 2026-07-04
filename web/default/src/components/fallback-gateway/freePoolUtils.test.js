import {
  buildFreeProviderRows,
  isAutoFreeDeploymentId,
  isAutoFreeDeployment,
  providerFromDeploymentId,
} from './freePoolUtils';

describe('freePoolUtils', () => {
  it('只把后端自动生成格式识别为自动免费部署', () => {
    expect(isAutoFreeDeploymentId('free:openrouter-1')).toBe(true);
    expect(isAutoFreeDeploymentId('free:groq-a1b2c3d4')).toBe(true);
    expect(isAutoFreeDeploymentId('free:google-a1b2c3d4')).toBe(true);
    expect(isAutoFreeDeploymentId('free:aihorde-1')).toBe(true);
    expect(isAutoFreeDeploymentId('free:nvidia-deadbeef')).toBe(true);
    expect(isAutoFreeDeploymentId('free:custom-model')).toBe(false);
    expect(isAutoFreeDeploymentId('paid_high-model')).toBe(false);
  });

  it('按 pool=free 或自动部署 ID 判断自动免费部署', () => {
    expect(isAutoFreeDeployment('manual-id', { pool: 'free' })).toBe(true);
    expect(isAutoFreeDeployment('free:openrouter-a1b2c3d4', { pool: 'cheap' })).toBe(true);
    expect(isAutoFreeDeployment('normal-dep', { pool: 'cheap' })).toBe(false);
  });

  it('从自动部署 ID 提取供应商', () => {
    expect(providerFromDeploymentId('free:openrouter-a1b2c3d4')).toBe('openrouter');
    expect(providerFromDeploymentId('free:groq-22')).toBe('groq');
    expect(providerFromDeploymentId('free:google-a1b2c3d4')).toBe('google');
    expect(providerFromDeploymentId('free:aihorde-1')).toBe('aihorde');
    expect(providerFromDeploymentId('free:custom-model')).toBe('-');
  });

  it('merges provider catalog with saved free provider config', () => {
    const rows = buildFreeProviderRows(
      {
        groq: {
          enabled: true,
          key_count: 2,
          models: ['custom-groq-model'],
          limits_override: { rpm_limit: 10 },
        },
        legacy: {
          enabled: false,
          key_count: 1,
        },
      },
      [
        {
          name: 'groq',
          enabled: false,
          key_count: 0,
          requires_key: true,
          keyless: false,
          rpm_limit: 30,
          tpm_limit: 6000,
          model_fetch_mode: 'static',
          quirks: { default_user_agent: 'test-agent' },
        },
        {
          name: 'aihorde',
          enabled: false,
          key_count: 0,
          requires_key: false,
          keyless: true,
          model_fetch_mode: 'openai_compatible',
          quirks: { disable_stream: true },
        },
      ],
    );

    expect(rows.map((row) => row.name)).toEqual(['groq', 'aihorde', 'legacy']);

    const groq = rows.find((row) => row.name === 'groq');
    expect(groq.configured).toBe(true);
    expect(groq.enabled).toBe(true);
    expect(groq.key_count).toBe(2);
    expect(groq.models).toEqual(['custom-groq-model']);
    expect(groq.limits_override.rpm_limit).toBe(10);
    expect(groq.rpm_limit).toBe(30);
    expect(groq.quirks.default_user_agent).toBe('test-agent');

    const aihorde = rows.find((row) => row.name === 'aihorde');
    expect(aihorde.configured).toBe(false);
    expect(aihorde.keyless).toBe(true);
    expect(aihorde.quirks.disable_stream).toBe(true);

    const legacy = rows.find((row) => row.name === 'legacy');
    expect(legacy.configured).toBe(true);
    expect(legacy.key_count).toBe(1);
  });

  it('counts staged provider keys before the config is saved', () => {
    const rows = buildFreeProviderRows(
      {
        nvidia: {
          enabled: true,
          keys: ['nvapi-staged-key'],
        },
      },
      [{ name: 'nvidia', key_count: 0, requires_key: true }],
    );

    expect(rows[0].key_count).toBe(1);
  });
});
