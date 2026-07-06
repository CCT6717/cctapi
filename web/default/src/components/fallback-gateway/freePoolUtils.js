import { PROVIDER_DISPLAY } from './freeProviderDisplay';

export const AUTO_FREE_DEPLOYMENT_RE = /^free:([a-z0-9][a-z0-9_-]*)-(\d+|[a-f0-9]{8})$/i;

export const isAutoFreeDeploymentId = (id) =>
  AUTO_FREE_DEPLOYMENT_RE.test(String(id || ''));

export const providerFromDeploymentId = (id) => {
  const match = String(id || '').match(AUTO_FREE_DEPLOYMENT_RE);
  return match ? match[1].toLowerCase() : '-';
};

// 注意：这是"自动免费池体系"专用判定——只认 free:openrouter-x / free:groq-x
// 这种 recognized provider 的 id。跟 deploymentMeta.js 的通用 isFreeDeployment
// 是不同概念：后者接受任何 free:* 前缀和 pool==='free'。别混用。
export const isAutoFreeDeployment = (id, dep) =>
  dep?.pool === 'free' || isAutoFreeDeploymentId(id);

const normalizeProviderName = (name) => String(name || '').trim().toLowerCase();

const pickNumber = (...values) => {
  for (const value of values) {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return 0;
};

const pickBoolean = (...values) => {
  for (const value of values) {
    if (typeof value === 'boolean') return value;
  }
  return false;
};

const cloneArray = (value) => (Array.isArray(value) ? [...value] : []);

const USAGE_UNAVAILABLE_MESSAGE = '用量数据不可用。';

const applyLimitOverride = (base, overrides, field) => {
  if (!overrides || overrides[field] === undefined || overrides[field] === null || overrides[field] === '') {
    return base;
  }
  const parsed = Number(overrides[field]);
  return Number.isFinite(parsed) ? parsed : base;
};

export const buildFreeProviderRows = (freeProviders = {}, catalog = []) => {
  const providers = freeProviders && typeof freeProviders === 'object' ? freeProviders : {};
  const entries = Array.isArray(catalog) ? catalog : [];
  const rows = [];
  const seen = new Set();

  const buildRow = (name, saved = {}, meta = {}, configured = false) => {
    const limitsOverride = { ...(saved.limits_override || {}) };
    const rpmLimit = pickNumber(meta.rpm_limit, saved.default_rpm);
    const rpdLimit = pickNumber(meta.rpd_limit, saved.default_rpd);
    const tpmLimit = pickNumber(meta.tpm_limit, saved.default_tpm);
    const tpdLimit = pickNumber(meta.tpd_limit, saved.default_tpd);

    return {
      ...meta,
      ...saved,
      name,
      configured,
      enabled: pickBoolean(saved.enabled, meta.enabled),
      key_count: cloneArray(saved.keys).length > 0
        ? cloneArray(saved.keys).length
        : pickNumber(saved.key_count, meta.key_count),
      models: cloneArray(saved.models).length > 0 ? cloneArray(saved.models) : cloneArray(meta.models),
      default_models: cloneArray(saved.default_models).length > 0
        ? cloneArray(saved.default_models)
        : cloneArray(meta.default_models),
      requires_key: pickBoolean(saved.requires_key, meta.requires_key),
      keyless: pickBoolean(saved.keyless, meta.keyless),
      supports_vision: pickBoolean(saved.supports_vision, meta.supports_vision),
      supports_stream: pickBoolean(saved.supports_stream, meta.supports_stream),
      supports_tools: pickBoolean(saved.supports_tools, meta.supports_tools),
      supports_json: pickBoolean(saved.supports_json, meta.supports_json),
      rpm_limit: applyLimitOverride(rpmLimit, limitsOverride, 'rpm_limit'),
      rpd_limit: applyLimitOverride(rpdLimit, limitsOverride, 'rpd_limit'),
      tpm_limit: applyLimitOverride(tpmLimit, limitsOverride, 'tpm_limit'),
      tpd_limit: applyLimitOverride(tpdLimit, limitsOverride, 'tpd_limit'),
      limits_override: limitsOverride,
      quirks: saved.quirks || meta.quirks || null,
      model_fetch_mode: saved.model_fetch_mode || meta.model_fetch_mode || '',
      provider_id: saved.provider_id || meta.provider_id || name,
      default_base_url: saved.default_base_url || meta.default_base_url || '',
    };
  };

  entries.forEach((entry) => {
    const name = normalizeProviderName(entry.name);
    if (!name) return;
    const saved = providers[name] || {};
    rows.push(buildRow(name, saved, entry, Object.prototype.hasOwnProperty.call(providers, name)));
    seen.add(name);
  });

  Object.keys(providers)
    .map(normalizeProviderName)
    .filter((name) => name && !seen.has(name))
    .sort()
    .forEach((name) => {
      rows.push(buildRow(name, providers[name], {}, true));
    });

  return rows;
};

export const isFreeProviderReady = (provider = {}) =>
  provider.enabled === true
  && (provider.keyless === true || Number(provider.key_count || 0) > 0);

export const freeProviderNeedsKey = (provider = {}) =>
  provider.requires_key === true
  && provider.keyless !== true
  && Number(provider.key_count || 0) <= 0;

const capabilityFilterField = (capability) => ({
  keyless: 'keyless',
  stream: 'supports_stream',
  tools: 'supports_tools',
  json: 'supports_json',
  vision: 'supports_vision',
})[capability] || '';

const providerSearchHaystack = (provider = {}) => {
  const capabilityTerms = [
    provider.keyless ? 'keyless 免 key' : '',
    provider.requires_key ? 'key required needs key 需要 key' : '',
    provider.supports_stream ? 'stream 流式' : '',
    provider.supports_tools ? 'tools tool calling 工具' : '',
    provider.supports_json ? 'json' : '',
    provider.supports_vision ? 'vision image 视觉' : '',
    provider.configured ? 'configured 已配置' : 'catalog 目录',
  ];
  return [
    provider.name,
    PROVIDER_DISPLAY[provider.name]?.title,
    provider.provider_id,
    provider.default_base_url,
    provider.model_fetch_mode,
    ...(Array.isArray(provider.models) ? provider.models : []),
    ...(Array.isArray(provider.default_models) ? provider.default_models : []),
    ...capabilityTerms,
  ]
    .filter(Boolean)
    .join(' ')
    .toLowerCase();
};

export const matchesFreeProviderFilters = (provider = {}, filters = {}) => {
  const searchText = String(filters.searchText || '').trim().toLowerCase();
  if (searchText && !providerSearchHaystack(provider).includes(searchText)) {
    return false;
  }

  switch (filters.statusFilter || 'all') {
    case 'enabled':
      if (provider.enabled !== true) return false;
      break;
    case 'disabled':
      if (provider.enabled === true) return false;
      break;
    case 'ready':
      if (!isFreeProviderReady(provider)) return false;
      break;
    case 'needs_key':
      if (!freeProviderNeedsKey(provider)) return false;
      break;
    case 'configured':
      if (provider.configured !== true) return false;
      break;
    case 'catalog':
      if (provider.configured === true) return false;
      break;
    default:
      break;
  }

  const capabilityField = capabilityFilterField(filters.capabilityFilter);
  if (capabilityField && provider[capabilityField] !== true) {
    return false;
  }

  return true;
};

export const filterFreeProviderRows = (rows = [], filters = {}) =>
  (Array.isArray(rows) ? rows : []).filter((provider) =>
    matchesFreeProviderFilters(provider, filters)
  );

export const buildBulkEnabledProviderConfig = (
  freeProviders = {},
  providers = [],
  enabled = true,
) => {
  const current = freeProviders && typeof freeProviders === 'object' ? freeProviders : {};
  const next = { ...current };
  (Array.isArray(providers) ? providers : []).forEach((provider) => {
    const name = normalizeProviderName(provider?.name || provider);
    if (!name) return;
    next[name] = {
      ...(current[name] || {}),
      enabled: enabled === true,
    };
  });
  return next;
};

export const buildFreePoolWorkflowSummary = ({
  config = {},
  freeDeployments = [],
  usageRows = [],
  usageAvailable = true,
} = {}) => {
  const freeModel = config?.virtual_models?.['cct/free'];
  const providerRows = buildFreeProviderRows(
    config?.free_providers || {},
    config?.free_provider_catalog || [],
  );
  const readyProviders = providerRows.filter((provider) =>
    provider.enabled && (provider.keyless || Number(provider.key_count || 0) > 0)
  );
  const enabledDeployments = (Array.isArray(freeDeployments) ? freeDeployments : [])
    .filter((deployment) => deployment.enabled !== false);
  const invalidRuntimeCount = enabledDeployments.filter((deployment) =>
    ['invalid', 'critical', 'failed'].includes(String(deployment.runtime?.health || '').toLowerCase())
  ).length;

  const hasFreeModel = !!freeModel;
  const virtualModelReady = hasFreeModel && freeModel.enabled !== false;
  const providerReady = readyProviders.length > 0;
  const deploymentReady = enabledDeployments.length > 0;
  const telemetryReady = usageAvailable !== false;
  const readinessChecks = [
    virtualModelReady,
    providerReady,
    deploymentReady,
    telemetryReady,
  ];
  const readinessScore = readinessChecks.filter(Boolean).length;
  const risks = [];
  const nextActions = [];

  if (!hasFreeModel) {
    risks.push({
      key: 'virtual-model-missing',
      level: 'critical',
      text: '缺少 cct/free 虚拟模型。',
    });
    nextActions.push('创建 cct/free 虚拟模型。');
  } else if (!virtualModelReady) {
    risks.push({
      key: 'virtual-model-disabled',
      level: 'critical',
      text: 'cct/free 虚拟模型已停用。',
    });
    nextActions.push('启用 cct/free 虚拟模型。');
  }

  if (!providerReady) {
    risks.push({
      key: 'no-ready-provider',
      level: 'critical',
      text: '没有已启用且可用的供应商，需要已保存密钥或支持免 key。',
    });
    nextActions.push('至少启用一个已保存密钥或支持免 key 的供应商。');
  }

  if (!deploymentReady) {
    risks.push({
      key: 'no-enabled-deployment',
      level: 'critical',
      text: '没有已启用的自动生成免费部署。',
    });
    nextActions.push('同步免费池以生成部署。');
  }

  if (invalidRuntimeCount > 0) {
    risks.push({
      key: 'runtime-invalid',
      level: 'warning',
      text: `${invalidRuntimeCount} 个自动生成部署报告运行状态异常。`,
    });
    nextActions.push('在生产流量进入前检查异常的自动生成部署。');
  }

  if (!telemetryReady) {
    risks.push({
      key: 'usage-unavailable',
      level: 'warning',
      text: '用量统计不可用。',
    });
    nextActions.push('检查免费池用量接口，再依赖额度信号。');
  }

  if (nextActions.length === 0 || usageRows.length > 0) {
    nextActions.push('通过 cct/free 发送一次小请求，并观察用量是否增长。');
  }

  return {
    readinessScore,
    readinessTotal: readinessChecks.length,
    readinessPercent: Math.round((readinessScore / readinessChecks.length) * 100),
    statusTone: risks.length > 0 ? 'warning' : readinessScore === readinessChecks.length ? 'success' : 'info',
    statusText: risks.length > 0
      ? `还有 ${risks.length} 项需要处理`
      : readinessScore === readinessChecks.length
        ? '免费池已就绪'
        : '免费池接入中',
    providerCount: providerRows.length,
    readyProviderCount: readyProviders.length,
    deploymentCount: Array.isArray(freeDeployments) ? freeDeployments.length : 0,
    enabledDeploymentCount: enabledDeployments.length,
    usageRowCount: Array.isArray(usageRows) ? usageRows.length : 0,
    invalidRuntimeCount,
    risks,
    nextActions,
    steps: [
      {
        key: 'virtual-model',
        label: 'cct/free 虚拟模型',
        complete: virtualModelReady,
        detail: virtualModelReady ? '已启用' : '需要处理',
      },
      {
        key: 'providers',
        label: '就绪供应商',
        complete: providerReady,
        detail: `${readyProviders.length} / ${providerRows.length}`,
      },
      {
        key: 'deployments',
        label: '已生成部署',
        complete: deploymentReady,
        detail: `${enabledDeployments.length} / ${Array.isArray(freeDeployments) ? freeDeployments.length : 0}`,
      },
      {
        key: 'usage',
        label: '用量统计',
        complete: telemetryReady,
        detail: telemetryReady ? `${Array.isArray(usageRows) ? usageRows.length : 0} 条` : '不可用',
      },
    ],
  };
};

export const indexUsageRows = (rows = []) => {
  const index = {};
  (Array.isArray(rows) ? rows : []).forEach((row) => {
    const provider = String(row.provider || '').trim().toLowerCase();
    const keyHash = String(row.key_hash || '').trim();
    if (!provider || !keyHash) return;
    const key = `${provider}:${keyHash}`;
    if (!index[key]) index[key] = [];
    index[key].push(row);
  });
  return index;
};

export const buildClearKeysProviderConfig = (freeProviders = {}, providerKey) => {
  const current = freeProviders && typeof freeProviders === 'object' ? freeProviders : {};
  const existing = current[providerKey] || {};
  const nextProvider = { ...existing, clear_keys: true };
  delete nextProvider.keys;
  return {
    ...current,
    [providerKey]: nextProvider,
  };
};

export const buildReplaceKeysProviderConfig = (freeProviders = {}, providerKey, value) => {
  const current = freeProviders && typeof freeProviders === 'object' ? freeProviders : {};
  const existing = current[providerKey] || {};
  const keys = String(value || '')
    .split(/\r?\n/)
    .map((key) => key.trim())
    .filter((key) => key && !key.includes('*'));
  const nextProvider = { ...existing, keys };
  delete nextProvider.clear_keys;
  return {
    ...current,
    [providerKey]: nextProvider,
  };
};

export const loadFreePoolDashboardData = async ({
  getGatewayConfig,
  getRuntimeStatus,
  getFreePoolUsage,
}) => {
  const usageRequest = getFreePoolUsage()
    .then((usageRes) => ({ response: usageRes, failed: false }))
    .catch(() => ({ response: null, failed: true }));
  const [configRes, runtimeRes, usageResult] = await Promise.all([
    getGatewayConfig(),
    getRuntimeStatus(),
    usageRequest,
  ]);
  const usageData = usageResult.response?.data || {};
  const usageAvailable = !usageResult.failed && usageData.success !== false;
  return {
    configData: configRes.data || {},
    runtimeData: runtimeRes.data || {},
    usageRows: usageAvailable && Array.isArray(usageData.data) ? usageData.data : [],
    usageAvailable,
    usageError: usageAvailable ? '' : USAGE_UNAVAILABLE_MESSAGE,
  };
};
