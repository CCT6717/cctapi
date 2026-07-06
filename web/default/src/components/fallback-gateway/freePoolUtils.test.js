import {
  buildClearKeysProviderConfig,
  buildBulkEnabledProviderConfig,
  buildFreePoolWorkflowSummary,
  buildFreeProviderRows,
  buildReplaceKeysProviderConfig,
  filterFreeProviderRows,
  indexUsageRows,
  isAutoFreeDeploymentId,
  isAutoFreeDeployment,
  isFreeProviderReady,
  freeProviderNeedsKey,
  loadFreePoolDashboardData,
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
    expect(groq.rpm_limit).toBe(10);
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
          keys: ['staged-key-placeholder'],
        },
      },
      [{ name: 'nvidia', key_count: 0, requires_key: true }],
    );

    expect(rows[0].key_count).toBe(1);
  });

  it('applies staged limit overrides to effective limits', () => {
    const rows = buildFreeProviderRows(
      {
        groq: {
          enabled: true,
          limits_override: {
            rpm_limit: 5,
            tpd_limit: 0,
          },
        },
      },
      [
        {
          name: 'groq',
          rpm_limit: 30,
          rpd_limit: 1000,
          tpm_limit: 6000,
          tpd_limit: 200000,
        },
      ],
    );

    expect(rows[0].rpm_limit).toBe(5);
    expect(rows[0].rpd_limit).toBe(1000);
    expect(rows[0].tpm_limit).toBe(6000);
    expect(rows[0].tpd_limit).toBe(0);
  });

  it('filters provider rows by search text, status, and capability', () => {
    const rows = buildFreeProviderRows(
      {
        groq: { enabled: true, key_count: 2 },
        openrouter: { enabled: false, key_count: 0 },
        aihorde: { enabled: true, key_count: 0 },
      },
      [
        {
          name: 'groq',
          supports_tools: true,
          model_fetch_mode: 'static',
          requires_key: true,
        },
        {
          name: 'openrouter',
          supports_json: true,
          requires_key: true,
        },
        {
          name: 'aihorde',
          keyless: true,
          supports_stream: true,
        },
      ],
    );

    expect(filterFreeProviderRows(rows, { searchText: 'tool' }).map((row) => row.name))
      .toEqual(['groq']);
    expect(filterFreeProviderRows(rows, { statusFilter: 'ready' }).map((row) => row.name))
      .toEqual(['groq', 'aihorde']);
    expect(filterFreeProviderRows(rows, { statusFilter: 'needs_key' }).map((row) => row.name))
      .toEqual(['openrouter']);
    expect(filterFreeProviderRows(rows, { capabilityFilter: 'keyless' }).map((row) => row.name))
      .toEqual(['aihorde']);
  });

  it('filters provider rows by display title text', () => {
    const rows = buildFreeProviderRows(
      {},
      [{ name: 'google', requires_key: true }],
    );

    expect(filterFreeProviderRows(rows, { searchText: 'google ai' }).map((row) => row.name))
      .toEqual(['google']);
  });

  it('classifies ready and needs-key providers', () => {
    expect(isFreeProviderReady({
      enabled: true,
      keyless: true,
      key_count: 0,
    })).toBe(true);
    expect(isFreeProviderReady({
      enabled: true,
      keyless: false,
      key_count: 1,
    })).toBe(true);
    expect(isFreeProviderReady({
      enabled: false,
      keyless: true,
      key_count: 0,
    })).toBe(false);
    expect(freeProviderNeedsKey({
      enabled: false,
      requires_key: true,
      keyless: false,
      key_count: 0,
    })).toBe(true);
  });

  it('stages bulk enabled-state updates without touching keys or limits', () => {
    const next = buildBulkEnabledProviderConfig(
      {
        groq: {
          enabled: true,
          key_count: 2,
          limits_override: { rpm_limit: 5 },
        },
        openrouter: {
          enabled: true,
          keys: ['staged-key'],
        },
      },
      [
        { name: 'groq' },
        { name: 'openrouter' },
        { name: 'aihorde' },
      ],
      false,
    );

    expect(next.groq.enabled).toBe(false);
    expect(next.groq.key_count).toBe(2);
    expect(next.groq.limits_override).toEqual({ rpm_limit: 5 });
    expect(next.openrouter.enabled).toBe(false);
    expect(next.openrouter.keys).toEqual(['staged-key']);
    expect(next.aihorde.enabled).toBe(false);
  });

  it('indexes usage rows by provider and key hash', () => {
    const index = indexUsageRows([
      {
        provider: 'groq',
        key_hash: '001122ff',
        model_name: 'llama-free',
        total_tokens: 10,
        request_count: 2,
        success_count: 2,
      },
    ]);

    expect(index['groq:001122ff']).toEqual([
      expect.objectContaining({ model_name: 'llama-free', total_tokens: 10 }),
    ]);
  });

  it('builds explicit clear key payload without raw keys', () => {
    const next = buildClearKeysProviderConfig(
      { groq: { enabled: false, key_count: 1 } },
      'groq',
    );

    expect(next.groq.clear_keys).toBe(true);
    expect(next.groq.keys).toBeUndefined();
  });

  it('builds replacement keys after clear without keeping clear_keys', () => {
    const next = buildReplaceKeysProviderConfig(
      {
        groq: {
          enabled: true,
          key_count: 1,
          clear_keys: true,
        },
      },
      'groq',
      'replacement-key-one\n****stored-key\n replacement-key-two ',
    );

    expect(next.groq.keys).toEqual(['replacement-key-one', 'replacement-key-two']);
    expect(next.groq.clear_keys).toBeUndefined();
  });

  it('loads config and runtime data when usage endpoint fails', async () => {
    const result = await loadFreePoolDashboardData({
      getGatewayConfig: jest.fn().mockResolvedValue({
        data: { success: true, data: { free_providers: { groq: { enabled: true } } } },
      }),
      getRuntimeStatus: jest.fn().mockResolvedValue({
        data: { success: true, data: [{ deployment_id: 'free:groq-1' }] },
      }),
      getFreePoolUsage: jest.fn().mockRejectedValue(new Error('usage unavailable')),
    });

    expect(result.configData.data.free_providers.groq.enabled).toBe(true);
    expect(result.runtimeData.data).toEqual([{ deployment_id: 'free:groq-1' }]);
    expect(result.usageRows).toEqual([]);
    expect(result.usageAvailable).toBe(false);
    expect(result.usageError).toBe('用量数据不可用。');
  });

  it('reports usage unavailable when usage response is unsuccessful', async () => {
    const result = await loadFreePoolDashboardData({
      getGatewayConfig: jest.fn().mockResolvedValue({
        data: { success: true, data: { free_providers: {} } },
      }),
      getRuntimeStatus: jest.fn().mockResolvedValue({
        data: { success: true, data: [] },
      }),
      getFreePoolUsage: jest.fn().mockResolvedValue({
        data: { success: false, message: 'sanitized usage lookup failed' },
      }),
    });

    expect(result.usageRows).toEqual([]);
    expect(result.usageAvailable).toBe(false);
    expect(result.usageError).toBe('用量数据不可用。');
  });

  it('summarizes a ready free pool workflow', () => {
    const summary = buildFreePoolWorkflowSummary({
      config: {
        free_providers: {
          groq: { enabled: true, key_count: 2 },
          aihorde: { enabled: true, keyless: true },
        },
        free_provider_catalog: [
          { name: 'groq', requires_key: true },
          { name: 'aihorde', keyless: true },
        ],
        virtual_models: {
          'cct/free': { enabled: true, pools: ['free'], strategy: 'free_first' },
        },
      },
      freeDeployments: [
        { id: 'free:groq-1', enabled: true, runtime: { health: 'valid' } },
        { id: 'free:aihorde-1', enabled: true, runtime: { health: 'valid' } },
      ],
      usageRows: [{ provider: 'groq', total_tokens: 120, request_count: 3 }],
      usageAvailable: true,
    });

    expect(summary.readinessScore).toBe(4);
    expect(summary.readinessTotal).toBe(4);
    expect(summary.readyProviderCount).toBe(2);
    expect(summary.enabledDeploymentCount).toBe(2);
    expect(summary.risks).toEqual([]);
    expect(summary.nextActions).toContain('通过 cct/free 发送一次小请求，并观察用量是否增长。');
  });

  it('reports missing workflow pieces as risks and next actions', () => {
    const summary = buildFreePoolWorkflowSummary({
      config: {
        free_providers: {
          groq: { enabled: true, key_count: 0 },
        },
        free_provider_catalog: [{ name: 'groq', requires_key: true }],
        virtual_models: {
          'cct/free': { enabled: false, pools: ['free'], strategy: 'free_first' },
        },
      },
      freeDeployments: [],
      usageRows: [],
      usageAvailable: false,
    });

    expect(summary.readinessScore).toBe(0);
    expect(summary.risks.map((risk) => risk.key)).toEqual([
      'virtual-model-disabled',
      'no-ready-provider',
      'no-enabled-deployment',
      'usage-unavailable',
    ]);
    expect(summary.nextActions).toEqual([
      '启用 cct/free 虚拟模型。',
      '至少启用一个已保存密钥或支持免 key 的供应商。',
      '同步免费池以生成部署。',
      '检查免费池用量接口，再依赖额度信号。',
    ]);
  });
});
